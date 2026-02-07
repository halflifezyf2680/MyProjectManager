use clap::Parser;
use ignore::WalkBuilder;
use rayon::prelude::*;
use rusqlite::{params, Connection, Result, OptionalExtension};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::{Arc, mpsc};
use std::time::{SystemTime, UNIX_EPOCH};
use tree_sitter::{Language, Parser as TsParser, Query, QueryCursor};

// ============================================================================
// CLI Arguments
// ============================================================================
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// Project root path
    #[arg(short, long)]
    project: String,

    /// Database path (symbols.db)
    #[arg(short, long)]
    db: String,

    /// Mode: index, map, query, structure, analyze, snapshot, diff
    #[arg(short, long, default_value = "index")]
    mode: String,

    /// Query string (for query mode)
    #[arg(short, long)]
    query: Option<String>,

    /// Extensions to include (comma separated)
    #[arg(short, long)]
    extensions: Option<String>,

    /// Output path for JSON result
    #[arg(short, long)]
    output: Option<String>,
    
    /// Directories to ignore (comma separated)
    #[arg(long)]
    ignore_dirs: Option<String>,

    /// Base snapshot path (for diff mode)
    #[arg(long)]
    base: Option<String>,

    /// Target snapshot path (for diff mode)
    #[arg(long)]
    target: Option<String>,

    /// File path for line-based symbol lookup (for query mode)
    #[arg(short, long)]
    file: Option<String>,

    /// Line number for symbol lookup (for query mode)
    #[arg(short, long)]
    line: Option<usize>,

    /// Scope path filter (for map mode)
    #[arg(long)]
    scope: Option<String>,

    /// Detail level: overview, standard, full (for map mode)
    #[arg(long, default_value = "standard")]
    detail: String,

    /// Analysis direction: forward, backward, both (for analyze mode)
    #[arg(long, default_value = "backward")]
    direction: String,
}

#[derive(Serialize)]
struct IndexResult {
    status: String,
    total_files: usize,
    elapsed_ms: u128,
}


// ============================================================================
// Data Models
// ============================================================================

struct ParseResult {
    file_path: String,
    file_hash: String,
    language: String,
    line_count: usize,
    symbols: Vec<PendingSymbol>,
    calls: Vec<PendingCall>,
}

struct PendingSymbol {
    temp_id: usize,
    parent_temp_id: Option<usize>,
    name: String,
    qualified_name: String,
    symbol_type: String,
    line_start: usize,
    line_end: usize,
    text: String,
    signature: Option<String>,  // 🆕 函数签名
}

struct PendingCall {
    caller_temp_id: usize,
    callee_name: String,
    line: usize,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct Node {
    id: String,
    #[serde(rename = "type")]
    node_type: String,
    name: String,
    qualified_name: String,
    file_path: String,
    line_start: usize,
    line_end: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    signature: Option<String>,
    #[serde(default)]
    calls: Vec<String>,
}

// ============================================================================
// Database & Indexer
// ============================================================================

fn init_db(conn: &Connection) -> Result<()> {
    conn.execute(
        "CREATE TABLE IF NOT EXISTS files (
            file_id INTEGER PRIMARY KEY AUTOINCREMENT,
            file_path TEXT UNIQUE NOT NULL,
            file_hash TEXT NOT NULL,
            language TEXT DEFAULT 'unknown',
            line_count INTEGER DEFAULT 0,
            updated_at INTEGER NOT NULL
        )",
        [],
    )?;

    // 🆕 添加 canonical_id 字段用于直接存储规范字符串 ID
    // 格式：func:src/core/session.py::get_task
    // 这样可以消除 Python 端的 ID 转换逻辑
    conn.execute(
        "CREATE TABLE IF NOT EXISTS symbols (
            symbol_id INTEGER PRIMARY KEY AUTOINCREMENT,
            file_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            qualified_name TEXT NOT NULL,
            canonical_id TEXT NOT NULL,
            symbol_type TEXT NOT NULL,
            line_start INTEGER,
            line_end INTEGER,
            signature TEXT,
            parent_id INTEGER,
            FOREIGN KEY (file_id) REFERENCES files(file_id) ON DELETE CASCADE
        )",
        [],
    )?;

    conn.execute(
        "CREATE TABLE IF NOT EXISTS calls (
            call_id INTEGER PRIMARY KEY AUTOINCREMENT,
            caller_id INTEGER NOT NULL,
            callee_name TEXT NOT NULL,
            call_line INTEGER,
            FOREIGN KEY (caller_id) REFERENCES symbols(symbol_id) ON DELETE CASCADE
        )",
        [],
    )?;
    
    // Performance Indices
    conn.execute("CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_id)", [])?;
    conn.execute("CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name)", [])?;
    conn.execute("CREATE INDEX IF NOT EXISTS idx_symbols_qname ON symbols(qualified_name)", [])?;
    conn.execute("CREATE INDEX IF NOT EXISTS idx_calls_caller ON calls(caller_id)", [])?;
    conn.execute("CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name)", [])?;

    Ok(())
}

fn calculate_hash(path: &Path) -> std::io::Result<String> {
    let mut file = fs::File::open(path)?;
    let mut hasher = Sha256::new();
    std::io::copy(&mut file, &mut hasher)?;
    Ok(hex::encode(hasher.finalize()))
}

fn main() -> anyhow::Result<()> {
    let args = Args::parse();
    let project_path = Path::new(&args.project);
    
    // Heartbeat setup
    let mcp_data = project_path.join(".mcp-data");
    let _ = fs::create_dir_all(&mcp_data);
    let heartbeat_path = mcp_data.join("heartbeat");

    if args.mode == "index" {
        run_indexer(&args, &heartbeat_path)?;
    } else if args.mode == "query" {
        run_query(&args)?;
    } else if args.mode == "map" {
        run_map(&args)?;
    } else if args.mode == "analyze" {
        run_analyze(&args)?;
    } else if args.mode == "snapshot" {
        run_snapshot(&args)?;
    } else if args.mode == "diff" {
        run_diff(&args)?;
    } else if args.mode == "structure" {
        run_structure(&args)?;
    }



    Ok(())
}

fn run_indexer(args: &Args, heartbeat_path: &Path) -> anyhow::Result<()> {
    println!("Starting indexer for: {}", args.project);
    
    // 1. Setup DB
    let mut conn = Connection::open(&args.db)?;
    init_db(&conn)?;
    
    // Optimizations
    conn.execute("PRAGMA synchronous = OFF", [])?;
    // PRAGMA journal_mode returns the new mode (string), so we must use query_row, not execute
    let _ : String = conn.query_row("PRAGMA journal_mode = WAL", [], |r| r.get(0)).unwrap_or_default();

    // 2. Discover Files
    let mut builder = WalkBuilder::new(&args.project);
    builder.hidden(false); // Process .git ? No, usually we want to ignore .git
    builder.git_ignore(true); // Respect .gitignore
    
    if let Some(ignores) = &args.ignore_dirs {
        let ignore_set: HashSet<String> = ignores.split(',').map(|s| s.trim().to_string()).collect();
        builder.filter_entry(move |entry| {
            if !entry.file_type().map(|f| f.is_dir()).unwrap_or(false) {
                return true;
            }
            !ignore_set.contains(entry.file_name().to_str().unwrap_or(""))
        });
    }

    
    let allowed_exts: HashSet<String> = args.extensions.as_ref()
        .map(|s| s.split(',').map(|ext| ext.trim().trim_start_matches('.').to_string()).collect())
        .unwrap_or_default();

    println!("Scanning directory...");
    let entries: Vec<PathBuf> = builder.build()
        .filter_map(|e| e.ok())
        .filter(|e| e.file_type().map(|t| t.is_file()).unwrap_or(false))
        .map(|e| e.path().to_path_buf())
        .filter(|p| {
            if allowed_exts.is_empty() { return true; }
            p.extension().map(|e| allowed_exts.contains(e.to_str().unwrap_or(""))).unwrap_or(false)
        })
        .collect();
    
    println!("Found {} files", entries.len());
    
    // 3. Process Files (Linear for DB safety, Rayon can be used for parsing if we separate Read/Write)
    // To keep it simple and safe for MVP: Sync Loop but fast because Tree-sitter is fast.
    // Actually, simple Loop is fine for < 10k files.
    
    // 3. Setup Parsers (Init once per thread inside par_iter to be safe, or local init)
    // Actually, tree-sitter parsers are cheap. We can init inside the loop.
    // Ideally we share `Query` objects as they are thread-safe (arc reference counting in rust wrapping?)
    // `tree_sitter::Query` is Send+Sync? Let's check docs. Yes usually.
    // The `Language` is just a pointer.
    
    // We'll prepare the Query map in main thread, and pass ref to workers.
    let parsers_setup = get_parser_setup(); 
    // parser_setup is HashMap<String, (Language, Query)>
    // Query is not cloneable easily? It is.
    // We wrap it in Arc for cheap sharing.
    let parsers_arc = Arc::new(parsers_setup);

    println!("Found {} files", entries.len());
    
    // 4. Pre-load Hashes (Optimization)
    let mut db_hashes: HashMap<String, String> = HashMap::new();
    {
        let mut stmt = conn.prepare("SELECT file_path, file_hash FROM files")?;
        let rows = stmt.query_map([], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)))?;
        for r in rows {
            if let Ok((path, hash)) = r {
                db_hashes.insert(path, hash);
            }
        }
    }
    
    let total = entries.len();
    
    // Channel for results
    let (tx_chan, rx_chan) = mpsc::channel::<ParseResult>();
    
    // 5. Parallel Processing
    // We use scoped thread or just rayon spawn. par_iter is blocking for the iterator, but we want to consume in main thread.
    // Pattern: `entries.par_iter().for_each_with(sender, ...)`
    // But `entries` needs to be moved or shared.
    
    // We can spawn a thread to drive the parallel processing, while main thread waits on RX.
    let entries_arc = Arc::new(entries);
    let db_hashes_arc = Arc::new(db_hashes);
    let project_root = args.project.clone();
    
    let producer_handle = std::thread::spawn(move || {
        entries_arc.par_iter().for_each(|path| {
            let path_str = path.strip_prefix(&project_root).unwrap_or(path).to_string_lossy().replace("\\", "/");
            
            // Calc Hash
            let content = match fs::read_to_string(path) {
                Ok(c) => c,
                Err(_) => return, // Skip binary or read error
            };
            
            let mut hasher = Sha256::new();
            hasher.update(content.as_bytes());
            let result = hasher.finalize();
            let new_hash = hex::encode(result);
            
            // Check Skip
            if let Some(old_hash) = db_hashes_arc.get(&path_str) {
                if *old_hash == new_hash {
                    // Start Heartbeat or Progress tick?
                    // We can send a "Skipped" message if we want fine grained progress, 
                    // but for now we just skip. 
                    // To update progress bar we might want to send a dummy result?
                    // Let's send a Empty result to count progress.
                    let _ = tx_chan.send(ParseResult {
                        file_path: path_str,
                        file_hash: new_hash, // Same hash
                        language: "skip".into(),
                        line_count: 0,
                        symbols: vec![],
                        calls: vec![],
                    });
                    return;
                }
            }
            
            // Parse
            let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("").to_lowercase();
            if let Some((lang, query)) = parsers_arc.get(&ext) {
                let mut parser = TsParser::new();
                parser.set_language(*lang).unwrap();
                
                let tree = parser.parse(&content, None).unwrap(); // handle err?
                
                let mut cursor = QueryCursor::new();
                let matches = cursor.matches(query, tree.root_node(), content.as_bytes());
                
                let mut symbols = vec![];
                let mut calls = vec![];
                let mut node_id_map: HashMap<usize, usize> = HashMap::new(); // tree_node_id -> temp_id
                let mut temp_counter = 0;
                
                for m in matches {
                    let mut node_name: Option<String> = None;
                    let mut node_type: Option<&str> = None;
                    let mut def_node: Option<tree_sitter::Node> = None;
                    let mut name_node: Option<tree_sitter::Node> = None;
                    let mut callee_node: Option<tree_sitter::Node> = None;

                    for capture in m.captures {
                        let capture_name = &query.capture_names()[capture.index as usize];
                        match capture_name.as_str() {
                            "name" => {
                                name_node = Some(capture.node);
                                node_name = Some(content[capture.node.start_byte()..capture.node.end_byte()].to_string());
                            },
                            "callee" => {
                                callee_node = Some(capture.node);
                            },
                            "def.func" => {
                                node_type = Some("function");
                                def_node = Some(capture.node);
                            },
                            "def.class" => {
                                node_type = Some("class");
                                def_node = Some(capture.node);
                            },
                            "ref.call" => {
                                // Already handled by callee?
                            },
                            _ => {}
                        }
                    }

                    if let (Some(name), Some(kind), Some(full_node)) = (node_name, node_type, def_node) {
                        // Definition
                        let start = full_node.start_position().row + 1;
                        let end = full_node.end_position().row + 1;
                        
                        temp_counter += 1;
                        let tid = temp_counter;
                        node_id_map.insert(full_node.id(), tid);
                        
                        // Find parent
                        let mut parent_temp_id = None;
                        let mut p_cursor = full_node.parent();
                        while let Some(p) = p_cursor {
                            if let Some(pid) = node_id_map.get(&p.id()) {
                                parent_temp_id = Some(*pid);
                                break;
                            }
                            p_cursor = p.parent();
                        }
                        
                        symbols.push(PendingSymbol {
                            temp_id: tid,
                            parent_temp_id,
                            name: name.clone(),
                            qualified_name: name.clone(),
                            symbol_type: kind.to_string(),
                            line_start: start,
                            line_end: end,
                            text: name,
                            signature: if kind == "function" {
                                let sig_text = &content[full_node.start_byte()..full_node.end_byte()];
                                sig_text.lines().next().map(|s| s.trim().to_string())
                            } else {
                                None
                            },
                        });
                    } else if let Some(c_node) = callee_node {
                        // Call
                        let callee_name = content[c_node.start_byte()..c_node.end_byte()].to_string();
                        // Find caller
                        let mut p_cursor = c_node.parent();
                        let mut caller_tid = 0;
                        let line = c_node.start_position().row + 1;
                        
                        while let Some(p) = p_cursor {
                            if let Some(pid) = node_id_map.get(&p.id()) {
                                caller_tid = *pid;
                                break;
                            }
                            p_cursor = p.parent();
                        }
                        
                        if caller_tid > 0 {
                            calls.push(PendingCall {
                                caller_temp_id: caller_tid,
                                callee_name,
                                line,
                            });
                        }
                    }
                }

                let line_count = content.lines().count();

                let _ = tx_chan.send(ParseResult {
                    file_path: path_str,
                    file_hash: new_hash,
                    language: ext,
                    line_count,
                    symbols,
                    calls,
                });
            }
        });
    });
    
    // 6. Consumer (Main Thread)
    let mut tx = conn.transaction()?;
    
    // Prepared Statements
    let mut stmt_upsert_file = tx.prepare(
        "INSERT INTO files (file_path, file_hash, language, line_count, updated_at) 
         VALUES (?1, ?2, ?3, ?4, ?5)
         ON CONFLICT(file_path) DO UPDATE SET file_hash=?2, updated_at=?5"
    )?;
    
    let mut stmt_del_symbols = tx.prepare("DELETE FROM symbols WHERE file_id = ?1")?;
    
    // 🆕 修改：包含 canonical_id 和 signature 字段
    let mut stmt_ins_symbol = tx.prepare(
        "INSERT INTO symbols (file_id, name, qualified_name, canonical_id, symbol_type, line_start, line_end, signature)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)"
    )?;


    let mut stmt_ins_call = tx.prepare(
        "INSERT INTO calls (caller_id, callee_name, call_line) VALUES (?1, ?2, ?3)"
    )?;
    
    let mut processed_count = 0;
    
    // Process results
    for res in rx_chan {
        processed_count += 1;
        
        // Heartbeat
        if processed_count % 10 == 0 {
             let json = format!(r#"{{"timestamp": {}, "processed": {}, "total": {}}}"#, 
                SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_secs(), 
                processed_count, total);
             let _ = fs::write(heartbeat_path, json);
        }
        
        // Handle Skip
        if res.language == "skip" {
            continue;
        }
        
        let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_secs();

        // 1. Upsert File
        stmt_upsert_file.execute(params![
            res.file_path, res.file_hash, res.language, res.line_count, now
        ])?;
        
        // Get File ID
        // Problem: Upsert doesn't always return ID in sqlite < 3.35 with RETURNING
        // We can look it up. Or use last_insert_rowid if it was an insert.
        // Or select it. Selection is safer.
        // Actually since we have a unique path, just select.
        // Optimization: In a massive transaction, Select might be slow?
        // We can cache file_id in memory if needed. 
        // Let's do a quick select.
        let file_id: i64 = tx.query_row("SELECT file_id FROM files WHERE file_path = ?1", 
            [&res.file_path], |r| r.get(0))?;
            
        // 2. Clear Old Symbols
        stmt_del_symbols.execute(params![file_id])?;
        
        // 3. Insert Symbols
        // We need to map temp_id -> realtime_db_id
        let mut temp_to_db_id: HashMap<usize, i64> = HashMap::new();

        for sym in &res.symbols {
            // 🆕 构建规范 canonical_id：type:file_path::name
            // 例如：func:src/core/session.py::get_task
            let prefix = if sym.symbol_type == "class" { "class" } else { "func" };
            let canonical_id = format!("{}:{}::{}", prefix, res.file_path, sym.name);

            stmt_ins_symbol.execute(params![
                file_id, sym.name, sym.qualified_name, canonical_id, sym.symbol_type, sym.line_start, sym.line_end, sym.signature
            ])?;

            let db_id = tx.last_insert_rowid();
            temp_to_db_id.insert(sym.temp_id, db_id);
        }
        
        // 4. Insert Calls
        for call in &res.calls {
            if let Some(caller_db_id) = temp_to_db_id.get(&call.caller_temp_id) {
                stmt_ins_call.execute(params![*caller_db_id, call.callee_name, call.line])?;
            }
        }
    }
    
    producer_handle.join().unwrap(); // Wait for producer to finish (should be done if channel closed)
    
    drop(stmt_upsert_file);
    drop(stmt_del_symbols);
    drop(stmt_ins_symbol);
    drop(stmt_ins_call);
    
    // ========================================================================
    // 🆕 Phase: Clean up deleted files (增量清理阶段)
    // 删除数据库中存在但文件系统中已不存在的文件记录
    // ========================================================================
    {
        let project_path = Path::new(&args.project);
        let mut stmt = tx.prepare("SELECT file_id, file_path FROM files")?;
        let rows: Vec<(i64, String)> = stmt.query_map([], |row| {
            Ok((row.get::<_, i64>(0)?, row.get::<_, String>(1)?))
        })?.filter_map(|r| r.ok()).collect();
        
        let mut deleted_count = 0;
        for (file_id, rel_path) in rows {
            let full_path = project_path.join(&rel_path);
            if !full_path.exists() {
                // File was deleted from filesystem, remove from index
                tx.execute("DELETE FROM symbols WHERE file_id = ?1", params![file_id])?;
                tx.execute("DELETE FROM files WHERE file_id = ?1", params![file_id])?;
                deleted_count += 1;
            }
        }
        
        if deleted_count > 0 {
            println!("[Cleanup] Removed {} stale file entries from index", deleted_count);
        }
    }
    
    tx.commit()?;
    
    println!("Indexing completed. Processed {} files.", processed_count);
    // Write Output
    if let Some(out_path) = &args.output {
        let result = IndexResult {
            status: "success".into(),
            total_files: total,
            elapsed_ms: 0,
        };
        let f = fs::File::create(out_path)?;
        serde_json::to_writer(f, &result)?;
    }
    
    Ok(())
}

#[derive(Serialize)]
struct QueryResult {
    status: String,
    query: String,
    found_symbol: Option<Node>,
    match_type: Option<String>,       // 🆕 匹配类型：exact/prefix_suffix/substring/levenshtein/stem
    candidates: Vec<CandidateMatch>,  // 🆕 多候选列表
    related_nodes: Vec<CallerInfo>,
}

#[derive(Serialize)]
struct CandidateMatch {
    node: Node,
    match_type: String,
    score: f32,  // 相似度分数 (0-1)
}

#[derive(Serialize)]
struct CallerInfo {
    node: Node,
    call_type: String,
}

// ============================================================================
// Progressive Fallback Search (渐进式容错查询)
// ============================================================================
use strsim::levenshtein;

fn progressive_search(conn: &Connection, query_str: &str) -> Option<(Node, String)> {
    let (best, _, _) = progressive_search_multi(conn, query_str);
    best.map(|n| (n.0, n.1))
}

// 🆕 多候选渐进式搜索
fn progressive_search_multi(conn: &Connection, query_str: &str) -> (Option<(Node, String)>, Vec<CandidateMatch>, bool) {
    let mut candidates: Vec<CandidateMatch> = vec![];
    let max_candidates = 5;
    
    // Layer 1: 精确匹配 (score = 1.0)
    if let Some(node) = exact_match(conn, query_str) {
        return (Some((node, "exact".to_string())), candidates, true);
    }
    
    // Layer 2: 前缀/后缀匹配 (score = 0.9)
    let prefix_matches = prefix_suffix_match_multi(conn, query_str, max_candidates);
    for node in prefix_matches {
        candidates.push(CandidateMatch {
            node,
            match_type: "prefix_suffix".to_string(),
            score: 0.9,
        });
    }
    if !candidates.is_empty() {
        let best = candidates[0].node.clone();
        return (Some((best, "prefix_suffix".to_string())), candidates, true);
    }
    
    // Layer 3: 子串匹配 (score = 0.8)
    let substring_matches = substring_match_multi(conn, query_str, max_candidates);
    for node in substring_matches {
        candidates.push(CandidateMatch {
            node,
            match_type: "substring".to_string(),
            score: 0.8,
        });
    }
    if !candidates.is_empty() {
        let best = candidates[0].node.clone();
        return (Some((best, "substring".to_string())), candidates, true);
    }
    
    // Layer 4: 编辑距离匹配 (score based on distance)
    let lev_matches = levenshtein_match_multi(conn, query_str, 3, max_candidates);
    for (node, dist) in lev_matches {
        let score = 1.0 - (dist as f32 / 4.0);  // distance 0=1.0, 1=0.75, 2=0.5, 3=0.25
        candidates.push(CandidateMatch {
            node,
            match_type: format!("levenshtein_d{}", dist),
            score,
        });
    }
    if !candidates.is_empty() {
        let best = candidates[0].node.clone();
        return (Some((best, "levenshtein".to_string())), candidates, true);
    }
    
    // Layer 5: 词根匹配 (score = 0.5)
    let stem_matches = stem_match_multi(conn, query_str, max_candidates);
    for node in stem_matches {
        candidates.push(CandidateMatch {
            node,
            match_type: "stem".to_string(),
            score: 0.5,
        });
    }
    if !candidates.is_empty() {
        let best = candidates[0].node.clone();
        return (Some((best, "stem".to_string())), candidates, true);
    }
    
    (None, candidates, false)
}

// 🆕 修改：使用 canonical_id 而不是 symbol_id
fn exact_match(conn: &Connection, query: &str) -> Option<Node> {
    let mut stmt = conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name = ?1 LIMIT 1"
    ).ok()?;
    stmt.query_row([query], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).ok()
}

// 🆕 修改：使用 canonical_id
fn prefix_suffix_match(conn: &Connection, query: &str) -> Option<Node> {
    let prefix_pattern = format!("{}%", query);
    let suffix_pattern = format!("%{}", query);
    let mut stmt = conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 OR name LIKE ?2 LIMIT 1"
    ).ok()?;
    stmt.query_row([prefix_pattern, suffix_pattern], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).ok()
}

// 🆕 修改：使用 canonical_id
fn substring_match(conn: &Connection, query: &str) -> Option<Node> {
    let pattern = format!("%{}%", query);
    let mut stmt = conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 LIMIT 1"
    ).ok()?;
    stmt.query_row([pattern], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).ok()
}

// 🆕 修改：使用 canonical_id
fn levenshtein_match(conn: &Connection, query: &str, max_distance: usize) -> Option<Node> {
    // 获取所有符号名，在内存中计算编辑距离
    let mut stmt = conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id"
    ).ok()?;

    let mut best: Option<(Node, usize)> = None;
    let query_lower = query.to_lowercase();

    let rows = stmt.query_map([], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).ok()?;

    for r in rows {
        if let Ok(node) = r {
            let dist = levenshtein(&query_lower, &node.name.to_lowercase());
            if dist <= max_distance {
                if best.is_none() || dist < best.as_ref().unwrap().1 {
                    best = Some((node, dist));
                }
            }
        }
    }
    
    best.map(|(n, _)| n)
}

// 🆕 修改：使用 canonical_id
fn stem_match(conn: &Connection, query: &str) -> Option<Node> {
    // 简单词根：取前 4 个字符
    if query.len() < 4 {
        return None;
    }
    let stem = &query[..4];
    let pattern = format!("{}%", stem);
    let mut stmt = conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 LIMIT 5"
    ).ok()?;
    stmt.query_row([pattern], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).ok()
}

// ============================================================================
// Multi-Candidate Match Functions (多候选匹配函数)
// ============================================================================

// 🆕 修改：使用 canonical_id
fn prefix_suffix_match_multi(conn: &Connection, query: &str, limit: usize) -> Vec<Node> {
    let prefix_pattern = format!("{}%", query);
    let suffix_pattern = format!("%{}", query);
    let mut stmt = match conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 OR name LIKE ?2 LIMIT ?3"
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let rows = match stmt.query_map(params![prefix_pattern, suffix_pattern, limit as i64], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }) {
        Ok(r) => r,
        Err(_) => return vec![],
    };

    rows.filter_map(|r| r.ok()).collect()
}

// 🆕 修改：使用 canonical_id
fn substring_match_multi(conn: &Connection, query: &str, limit: usize) -> Vec<Node> {
    let pattern = format!("%{}%", query);
    let mut stmt = match conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 LIMIT ?2"
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let rows = match stmt.query_map(params![pattern, limit as i64], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }) {
        Ok(r) => r,
        Err(_) => return vec![],
    };

    rows.filter_map(|r| r.ok()).collect()
}

// 🆕 修改：使用 canonical_id
fn levenshtein_match_multi(conn: &Connection, query: &str, max_distance: usize, limit: usize) -> Vec<(Node, usize)> {
    let mut stmt = match conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id"
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let query_lower = query.to_lowercase();
    let mut matches: Vec<(Node, usize)> = vec![];

    let rows = match stmt.query_map([], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }) {
        Ok(r) => r,
        Err(_) => return vec![],
    };

    for r in rows {
        if let Ok(node) = r {
            let dist = levenshtein(&query_lower, &node.name.to_lowercase());
            if dist <= max_distance {
                matches.push((node, dist));
            }
        }
    }

    // 按距离排序
    matches.sort_by_key(|(_, d)| *d);
    matches.truncate(limit);
    matches
}

// 🆕 修改：使用 canonical_id
fn stem_match_multi(conn: &Connection, query: &str, limit: usize) -> Vec<Node> {
    if query.len() < 4 {
        return vec![];
    }
    let stem = &query[..4];
    let pattern = format!("{}%", stem);
    let mut stmt = match conn.prepare(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE name LIKE ?1 LIMIT ?2"
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let rows = match stmt.query_map(params![pattern, limit as i64], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }) {
        Ok(r) => r,
        Err(_) => return vec![],
    };
    
    rows.filter_map(|r| r.ok()).collect()
}


fn run_query(args: &Args) -> anyhow::Result<()> {
    let conn = Connection::open(&args.db)?;
    
    // 策略优先级：
    // 1. 如果有 file + line，按行号定位符号
    // 2. 如果有 query，使用模糊匹配
    
    let found: Option<Node>;
    let mut candidates: Vec<CandidateMatch> = vec![];
    let mut match_type_str: Option<String> = None;
    
    if let (Some(file_path), Some(line_num)) = (&args.file, &args.line) {
        // === 行号定位模式 ===
        // 找到包含该行的符号（line_start <= line <= line_end）
        let mut stmt = conn.prepare(
            "SELECT symbol_id, name, qualified_name, file_path, line_start, line_end, symbol_type 
             FROM symbols JOIN files ON symbols.file_id = files.file_id 
             WHERE file_path LIKE ?1 AND line_start <= ?2 AND line_end >= ?2
             ORDER BY (line_end - line_start) ASC
             LIMIT 1"
        )?;
        // 使用 LIKE 模糊匹配文件路径（支持相对路径）
        let file_pattern = format!("%{}", file_path.replace("\\", "/"));
        found = stmt.query_row(params![file_pattern, line_num], |row| {
            Ok(Node {
                id: row.get::<_, i64>(0)?.to_string(),
                name: row.get(1)?,
                qualified_name: row.get(2)?,
                file_path: row.get(3)?,
                line_start: row.get(4)?,
                line_end: row.get(5)?,
                node_type: row.get(6)?,
                signature: None,
                calls: vec![],
            })
        }).optional()?;
    } else if let Some(query_str) = &args.query {
        // === 渐进式容错匹配（多候选） ===
        let (best_match, cands, _success) = progressive_search_multi(&conn, query_str);
        found = best_match.clone().map(|(node, _)| node);
        candidates = cands;
        match_type_str = best_match.map(|(_, mt)| mt);
    } else {
        // 无查询条件
        found = None;
        candidates = vec![];
        match_type_str = None;
    }
    
    // 查找调用者（保持原有逻辑）
    let mut related = vec![];
    if let Some(ref sym) = found {
        let mut call_stmt = conn.prepare(
            "SELECT s.symbol_id, s.name, s.qualified_name, f.file_path, s.line_start, s.line_end, s.symbol_type 
             FROM calls c 
             JOIN symbols s ON c.caller_id = s.symbol_id 
             JOIN files f ON s.file_id = f.file_id
             WHERE c.callee_name = ?1"
        )?;
        
        let rows = call_stmt.query_map([&sym.name], |row| {
             Ok(CallerInfo {
                 node: Node {
                     id: row.get::<_, i64>(0)?.to_string(),
                     name: row.get(1)?,
                     qualified_name: row.get(2)?,
                     file_path: row.get(3)?,
                     line_start: row.get(4)?,
                     line_end: row.get(5)?,
                     node_type: row.get(6)?,
                     signature: None,
                     calls: vec![],
                 },
                 call_type: "direct".to_string()
             })
        })?;
        
        for r in rows {
            if let Ok(info) = r {
                related.push(info);
            }
        }
    }

    // 输出结果
    if let Some(out_path) = &args.output {
        let res = QueryResult {
            status: "success".to_string(),
            query: args.query.clone().unwrap_or_default(),
            found_symbol: found,
            match_type: match_type_str,
            candidates: candidates,
            related_nodes: related,
        };
        let f = fs::File::create(out_path)?;
        serde_json::to_writer(f, &res)?;
    }

    Ok(())
}

#[derive(Serialize)]
struct MapResult {
    statistics: Stats,
    structure: HashMap<String, Vec<Node>>,
    elapsed: String,
}

#[derive(Serialize, Default)]
struct Stats {
    total_files: usize,
    total_symbols: usize,
}

fn run_map(args: &Args) -> anyhow::Result<()> {
    let conn = Connection::open(&args.db)?;
    
    // Stats
    let mut stats = Stats::default();
    
    // Structure
    let mut structure: HashMap<String, Vec<Node>> = HashMap::new();
    
    // 🆕 修改：添加 canonical_id 和 signature 字段
    let sql_base = "SELECT file_path, name, qualified_name, symbol_type, line_start, line_end, canonical_id, signature FROM symbols JOIN files ON symbols.file_id = files.file_id";
    
    if let Some(scope) = &args.scope {
        if !scope.is_empty() {
             // === 有 Scope 过滤 ===
            let pattern = format!("{}%", scope.replace("\\", "/"));
            
            // Stats (Scoped)
            stats.total_files = conn.query_row("SELECT count(*) FROM files WHERE file_path LIKE ?1", [&pattern], |r| r.get(0)).unwrap_or(0);
            stats.total_symbols = conn.query_row("SELECT count(*) FROM symbols JOIN files ON symbols.file_id = files.file_id WHERE file_path LIKE ?1", [&pattern], |r| r.get(0)).unwrap_or(0);
            
            let sql = format!("{} WHERE file_path LIKE ?1", sql_base);
            let mut stmt = conn.prepare(&sql)?;
            let rows = stmt.query_map([&pattern], |row| {
                Ok((
                    row.get::<_, String>(0)?, // file_path
                    Node {
                        id: row.get::<_, String>(6)?, // 🆕 canonical_id as ID (规范字符串)
                        name: row.get(1)?,
                        qualified_name: row.get(2)?,
                        file_path: row.get(0)?,
                        line_start: row.get(4)?,
                        line_end: row.get(5)?,
                        node_type: row.get(3)?,
                        signature: row.get(7)?, // 🆕 从数据库读取签名
                        calls: vec![],
                    }
                ))
            })?;
            
            for r in rows {
                if let Ok((path, node)) = r {
                    structure.entry(path).or_default().push(node);
                }
            }
        } else {
             // === Scope 为空字符串，视为全量 ===
            stats.total_files = conn.query_row("SELECT count(*) FROM files", [], |r| r.get(0)).unwrap_or(0);
            stats.total_symbols = conn.query_row("SELECT count(*) FROM symbols", [], |r| r.get(0)).unwrap_or(0);
            
            let mut stmt = conn.prepare(sql_base)?;
            let rows = stmt.query_map([], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    Node {
                        id: row.get::<_, String>(6)?,
                        name: row.get(1)?,
                        qualified_name: row.get(2)?,
                        file_path: row.get(0)?,
                        line_start: row.get(4)?,
                        line_end: row.get(5)?,
                        node_type: row.get(3)?,
                        signature: row.get(7)?, // 🆕
                        calls: vec![],
                    }
                ))
            })?;
             for r in rows {
                if let Ok((path, node)) = r {
                    structure.entry(path).or_default().push(node);
                }
            }
        }
    } else {
         // === 无 Scope 参数，视为全量 ===
        stats.total_files = conn.query_row("SELECT count(*) FROM files", [], |r| r.get(0)).unwrap_or(0);
        stats.total_symbols = conn.query_row("SELECT count(*) FROM symbols", [], |r| r.get(0)).unwrap_or(0);
        
        let mut stmt = conn.prepare(sql_base)?;
        let rows = stmt.query_map([], |row| {
             Ok((
                row.get::<_, String>(0)?,
                Node {
                    id: row.get::<_, String>(6)?,
                    name: row.get(1)?,
                    qualified_name: row.get(2)?,
                    file_path: row.get(0)?,
                    line_start: row.get(4)?,
                    line_end: row.get(5)?,
                    node_type: row.get(3)?,
                    signature: row.get(7)?, // 🆕
                    calls: vec![],
                }
            ))
        })?;
         for r in rows {
            if let Ok((path, node)) = r {
                structure.entry(path).or_default().push(node);
            }
        }
    };

    if let Some(out_path) = &args.output {
        let res = MapResult {
            statistics: stats,
            structure,
            elapsed: "0s".to_string(),
        };
        let f = fs::File::create(out_path)?;
        serde_json::to_writer(f, &res)?;
    }
    
    Ok(())
}


fn get_parser_setup() -> HashMap<String, (Language, Query)> {
    let mut map = HashMap::new();
    
    // Python
    let py_lang = tree_sitter_python::language();
    let py_query = Query::new(py_lang, r#"
        (function_definition name: (identifier) @name) @def.func
        (class_definition name: (identifier) @name) @def.class
        (call function: (identifier) @callee) @ref.call
        (call function: (attribute attribute: (identifier) @callee)) @ref.call
    "#).expect("Invalid Python Query");
    map.insert("py".to_string(), (py_lang, py_query));

    // JS
    let js_lang = tree_sitter_javascript::language();
    let js_query_str = r#"
        (function_declaration name: (identifier) @name) @def.func
        (class_declaration name: (identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
        (call_expression function: (member_expression property: (property_identifier) @callee)) @ref.call
    "#;
    let js_query = Query::new(js_lang, js_query_str).expect("Invalid JS Query");
    map.insert("js".to_string(), (js_lang, js_query));

    // Node.js ES Modules (.mjs)
    let mjs_query = Query::new(js_lang, js_query_str).expect("Invalid JS Query");
    map.insert("mjs".to_string(), (js_lang, mjs_query));

    // Node.js CommonJS (.cjs)
    let cjs_query = Query::new(js_lang, js_query_str).expect("Invalid JS Query");
    map.insert("cjs".to_string(), (js_lang, cjs_query));

    // TypeScript (.ts, .tsx)
    let ts_lang = tree_sitter_typescript::language_typescript();
    let ts_query_str = r#"
        (function_declaration name: (identifier) @name) @def.func
        (class_declaration name: (type_identifier) @name) @def.class
        (method_definition name: (property_identifier) @name) @def.func
        (call_expression function: (identifier) @callee) @ref.call
        (call_expression function: (member_expression property: (property_identifier) @callee)) @ref.call
    "#;
    let ts_query = Query::new(ts_lang, ts_query_str).expect("Invalid TypeScript Query");
    map.insert("ts".to_string(), (ts_lang, ts_query));

    // TSX (TypeScript + JSX)
    let tsx_lang = tree_sitter_typescript::language_tsx();
    let tsx_query = Query::new(tsx_lang, ts_query_str).expect("Invalid TSX Query");
    map.insert("tsx".to_string(), (tsx_lang, tsx_query));
    
    // Go
    let go_lang = tree_sitter_go::language();
    let go_query = Query::new(go_lang, r#"
        (function_declaration name: (identifier) @name) @def.func
        (method_declaration name: (field_identifier) @name) @def.func
        (type_spec name: (type_identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
        (call_expression function: (selector_expression field: (field_identifier) @callee)) @ref.call
    "#).expect("Invalid Go Query");
    map.insert("go".to_string(), (go_lang, go_query));

    // Rust
    let rs_lang = tree_sitter_rust::language();
    let rs_query = Query::new(rs_lang, r#"
        (function_item name: (identifier) @name) @def.func
        (struct_item name: (type_identifier) @name) @def.class
        (enum_item name: (type_identifier) @name) @def.class
        (impl_item type: (type_identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
        (call_expression function: (scoped_identifier name: (identifier) @callee)) @ref.call
        (call_expression function: (field_expression field: (field_identifier) @callee)) @ref.call
    "#).expect("Invalid Rust Query");
    map.insert("rs".to_string(), (rs_lang, rs_query));

    // Java
    let java_lang = tree_sitter_java::language();
    let java_query = Query::new(java_lang, r#"
        (class_declaration name: (identifier) @name) @def.class
        (method_declaration name: (identifier) @name) @def.func
        (interface_declaration name: (identifier) @name) @def.class
        (method_invocation name: (identifier) @callee) @ref.call
    "#).expect("Invalid Java Query");
    map.insert("java".to_string(), (java_lang, java_query));

    // C
    let c_lang = tree_sitter_c::language();
    let c_query = Query::new(c_lang, r#"
        (function_definition declarator: (function_declarator declarator: (identifier) @name)) @def.func
        (struct_specifier name: (type_identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
    "#).expect("Invalid C Query");
    map.insert("c".to_string(), (c_lang, c_query));
    
    // Re-create query for headers (Query is not Clone)
    let c_query_h = Query::new(c_lang, r#"
        (function_definition declarator: (function_declarator declarator: (identifier) @name)) @def.func
        (struct_specifier name: (type_identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
    "#).expect("Invalid C Query");
    map.insert("h".to_string(), (c_lang, c_query_h));

    // C++
    let cpp_lang = tree_sitter_cpp::language();
    let cpp_query_str = r#"
        (function_definition declarator: (function_declarator declarator: (identifier) @name)) @def.func
        (class_specifier name: (type_identifier) @name) @def.class
        (struct_specifier name: (type_identifier) @name) @def.class
        (call_expression function: (identifier) @callee) @ref.call
        (call_expression function: (field_expression field: (field_identifier) @callee)) @ref.call
    "#;
    
    let cpp_query = Query::new(cpp_lang, cpp_query_str).expect("Invalid C++ Query");
    map.insert("cpp".to_string(), (cpp_lang, cpp_query));
    
    let cpp_query_cc = Query::new(cpp_lang, cpp_query_str).expect("Invalid C++ Query");
    map.insert("cc".to_string(), (cpp_lang, cpp_query_cc));
    
    let cpp_query_hpp = Query::new(cpp_lang, cpp_query_str).expect("Invalid C++ Query");
    map.insert("hpp".to_string(), (cpp_lang, cpp_query_hpp));

    // TODO: Kotlin, Swift, Ruby need tree-sitter version alignment
    // Blocked by: tree-sitter-kotlin/swift/ruby require ts 0.22+ but other grammars are on 0.20
    // Solution: Wait for all grammars to align, or fork/patch individual crates

    map
}


// ============================================================================
// Impact Analysis & Dice Algorithm (Rust Implementation)
// ============================================================================

#[derive(Serialize)]
struct AnalysisResult {
    status: String,
    node_id: String,
    complexity_score: f64,
    complexity_level: String,
    affected_nodes: usize,
    direct_callers: Vec<CallerInfo>,
    indirect_callers: Vec<CallerInfo>,
    risk_level: String,
    modification_checklist: Vec<String>,
}

// 🆕 修改：使用 canonical_id
fn run_analyze(args: &Args) -> anyhow::Result<()> {
    let conn = Connection::open(&args.db)?;
    let query_str = args.query.as_ref().expect("Query required for analysis");

    // 1. Locate Target Node (精确匹配优先，失败后模糊匹配)
    // 先尝试精确匹配
    let mut stmt = conn.prepare("SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type FROM symbols JOIN files ON symbols.file_id = files.file_id WHERE name = ?1 LIMIT 1")?;

    let target_node = stmt.query_row([query_str], |row| {
        Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    }).optional()?.or_else(|| {
        // 精确匹配失败，尝试模糊匹配
        let fuzzy_pattern = format!("%{}%", query_str);
        let mut fuzzy_stmt = conn.prepare(
            "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
             FROM symbols JOIN files ON symbols.file_id = files.file_id
             WHERE name LIKE ?1 OR qualified_name LIKE ?1
             LIMIT 1"
        ).ok()?;
        fuzzy_stmt.query_row([fuzzy_pattern], |row| {
            Ok(Node {
                id: row.get::<_, String>(0)?, // 🆕 canonical_id
                name: row.get(1)?,
                qualified_name: row.get(2)?,
                file_path: row.get(3)?,
                line_start: row.get(4)?,
                line_end: row.get(5)?,
                node_type: row.get(6)?,
                signature: None,
                calls: vec![],
            })
        }).ok()
    });
                    
    let target = match target_node {
        Some(n) => n,
        None => {
             // Return empty/error JSON
             if let Some(out_path) = &args.output {
                let err = serde_json::json!({"status": "error", "message": "Symbol not found"});
                let f = fs::File::create(out_path)?;
                serde_json::to_writer(f, &err)?;
             }
             return Ok(());
        }
    };

    // 🆕 target.id 现在是 canonical_id (String)，不再需要 parse
    let target_id: String = target.id;

    // 2. Build In-Memory Graph (Adjacency & Reverse Adjacency)
    // For Dice: we need Outgoing edges (Calls).
    // For Impact: we need Incoming edges (Called By).
    
    // Query all calls: caller_id -> callee_name. 
    // Wait, calls table stores callee_name (string). usage is loose.
    // To do precise graph, we need to map callee_name back to symbol_id.
    // This is the "Linking" phase. In MVP Rust indexer, we didn't resolve callee_name to ID during index.
    // So we have to do it now or loose matching.
    
    println!("Building dependency graph...");

    // 🆕 使用 canonical_id (String) 而不是 symbol_id (i64)
    // Load all symbols into Map: Name -> Vec<canonical_id>
    let mut name_to_ids: HashMap<String, Vec<String>> = HashMap::new();
    {
        let mut s = conn.prepare("SELECT canonical_id, name FROM symbols")?; // 🆕 canonical_id
        let rows = s.query_map([], |r| Ok((r.get::<_, String>(0)?, r.get::<_, String>(1)?)))?; // 🆕 String, String
        for r in rows {
            if let Ok((id, name)) = r {
                name_to_ids.entry(name).or_default().push(id);
            }
        }
    }

    // Load all calls
    // 🆕 使用 String (canonical_id) 而不是 i64 (symbol_id)
    let mut adjacency: HashMap<String, Vec<String>> = HashMap::new(); // Caller -> Callee(s)
    let mut reverse_adjacency: HashMap<String, Vec<String>> = HashMap::new(); // Callee -> Caller(s)
    
    {
        // 🆕 JOIN symbols 表获取 canonical_id
        let mut s = conn.prepare("SELECT s.canonical_id, c.callee_name FROM calls c JOIN symbols s ON c.caller_id = s.symbol_id")?;
        let rows = s.query_map([], |r| Ok((r.get::<_, String>(0)?, r.get::<_, String>(1)?)))?;
        for r in rows {
            if let Ok((caller_canonical_id, callee_name)) = r {
                if let Some(callee_ids) = name_to_ids.get(&callee_name) {
                    for callee_id in callee_ids {
                         adjacency.entry(caller_canonical_id.clone()).or_default().push(callee_id.clone());
                         reverse_adjacency.entry(callee_id.clone()).or_default().push(caller_canonical_id.clone());
                    }
                }
            }
        }
    }
    
    // 3. Impact Analysis (BFS)
    let mut direct_nodes = Vec::new();
    let mut indirect_nodes = Vec::new();
    let mut affected_nodes = HashSet::new();

    let direction = args.direction.to_lowercase();
    
    // 我们定义“主方向图”
    // 如果是 backward (影响分析)，我们需要找到“谁在调用我”，即使用 reverse_adjacency
    // 如果是 forward (依赖分析)，我们需要找到“我在调用谁”，即使用 adjacency
    let primary_graph = if direction == "forward" {
        &adjacency
    } else {
        &reverse_adjacency // 默认 backward
    };
    
    // Direct
    if let Some(nodes) = primary_graph.get(&target_id) {
        for cid in nodes {
            affected_nodes.insert(cid.clone());
            // Get Node Info
            let node = get_node_by_id(&conn, cid)?;
            direct_nodes.push(CallerInfo {
                node,
                call_type: "direct".to_string()
            });
        }
    }

    // Indirect (Depth 2-3) - BFS
    let mut queue: Vec<(String, usize)> = direct_nodes.iter().map(|c| (c.node.id.clone(), 1)).collect();
    let mut visited: HashSet<String> = HashSet::new();
    visited.insert(target_id.clone());
    for c in &direct_nodes { visited.insert(c.node.id.clone()); }

    while let Some((curr, depth)) = queue.pop() {
        if depth >= 3 { continue; }
        if let Some(nodes) = primary_graph.get(&curr) {
            for cid in nodes {
                if !visited.contains(cid) {
                    visited.insert(cid.clone());
                    affected_nodes.insert(cid.clone());
                    let node = get_node_by_id(&conn, cid)?;
                    indirect_nodes.push(CallerInfo {
                        node,
                        call_type: "indirect".to_string()
                    });
                    queue.push((cid.clone(), depth + 1));
                }
            }
        }
    }

    // 4. Dice Algorithm (Complexity Score via Random Walk)
    // Run random walk starting from target node on the DIRECT graph (forward).
    // "If I am complex, I call many things which call many things."
    use rand::prelude::IndexedRandom; // rand 0.9 fix

    // 🆕 使用 String (canonical_id) 而不是 i64 (symbol_id)
    let mut walk_visits: HashMap<String, u32> = HashMap::new();
    let num_walks = 1000;
    let walk_length = 10;
    let damping = 0.85;
    let mut rng = rand::rng(); // rand 0.9 fix

    for _ in 0..num_walks {
        let mut curr = target_id.clone();
        for _ in 0..walk_length {
             *walk_visits.entry(curr.clone()).or_insert(0) += 1;

             if rand::random::<f64>() > damping {
                 break;
             }

             match adjacency.get(&curr) {
                 Some(neighbors) if !neighbors.is_empty() => {
                     curr = neighbors.choose(&mut rng).unwrap().clone();
                 },
                 _ => break,
             }
        }
    }
    
    // Calculate Score
    // Scope (Affected Nodes in dependency chain) - actually Random Walk measures "Effort to understand dependencies".
    let coverage = walk_visits.len();
    
    // Density (Fan-out)
    let out_degree = adjacency.get(&target_id).map(|v| v.len()).unwrap_or(0);
    let in_degree = reverse_adjacency.get(&target_id).map(|v| v.len()).unwrap_or(0);
    
    // Formula from dice.py: (affected * 0.4) + (density * 0.3) + (variance * 0.3)
    // Simplify for Rust MVP
    let complexity_score = (coverage as f64 * 0.5) + (out_degree as f64 * 2.0) + (in_degree as f64 * 1.0);
    let normalized_score = if complexity_score > 100.0 { 100.0 } else { complexity_score };
    
    let complexity_level = if normalized_score < 20.0 { "Simple" }
        else if normalized_score < 50.0 { "Medium" }
        else if normalized_score < 80.0 { "High" }
        else { "Extreme" };

    // Risk Level (Only meaningful for backward)
    let total_affected = direct_nodes.len() + indirect_nodes.len();
    let risk_level = if total_affected == 0 { "low" }
        else if total_affected <= 3 { "low" }
        else if total_affected <= 10 { "medium" }
        else { "high" };
        
    // Generate Checklist
    let mut checklist = vec![
        format!("📌 Target Symbol: {} ({})", target.qualified_name, target.file_path)
    ];
    let label = if direction == "forward" { "Dependency" } else { "Caller" };
    for c in &direct_nodes {
         checklist.push(format!("⚠️ Check {}: {}:{} ({})", label, c.node.node_type, c.node.name, c.node.file_path));
    }


    let final_res = AnalysisResult {
        status: "success".to_string(),
        node_id: target_id,
        complexity_score: normalized_score,
        complexity_level: complexity_level.to_string(),
        affected_nodes: total_affected,
        direct_callers: direct_nodes,
        indirect_callers: indirect_nodes,
        risk_level: risk_level.to_string(),
        modification_checklist: checklist
    };
    
    if let Some(out_path) = &args.output {
         let f = fs::File::create(out_path)?;
         serde_json::to_writer(f, &final_res)?;
    }
    
    Ok(())
}


// 🆕 修改：使用 canonical_id (String) 而不是 symbol_id (i64)
fn get_node_by_id(conn: &Connection, id: &str) -> Result<Node> {
    conn.query_row(
        "SELECT canonical_id, name, qualified_name, file_path, line_start, line_end, symbol_type
         FROM symbols JOIN files ON symbols.file_id = files.file_id
         WHERE canonical_id = ?1",
        [id],
        |row| Ok(Node {
            id: row.get::<_, String>(0)?, // 🆕 canonical_id
            name: row.get(1)?,
            qualified_name: row.get(2)?,
            file_path: row.get(3)?,
            line_start: row.get(4)?,
            line_end: row.get(5)?,
            node_type: row.get(6)?,
            signature: None,
            calls: vec![],
        })
    )
}

// ============================================================================
// Snapshot & Diff
// ============================================================================

#[derive(Serialize, Deserialize)]
struct Snapshot {
    timestamp: u64,
    symbols: HashMap<String, SnapshotSymbol>, // key: qualified_name (or id if stable)
}

#[derive(Serialize, Deserialize, Debug, PartialEq)] // Added PartialEq for easy diff
struct SnapshotSymbol {
    name: String,
    qualified_name: String,
    file_path: String,
    symbol_type: String,
    line_start: usize,
    signature: Option<String>,
    calls: Vec<String>, // List of callee qualified_names
}

// 🆕 修改：使用 canonical_id
fn run_snapshot(args: &Args) -> anyhow::Result<()> {
    // Export current DB state to a JSON file
    let conn = Connection::open(&args.db)?;

    // 1. Load Symbols
    let mut symbols_map: HashMap<String, SnapshotSymbol> = HashMap::new();
    let mut id_to_qname: HashMap<String, String> = HashMap::new(); // 🆕 canonical_id -> qualified_name

    {
        // 🆕 查询包含 canonical_id
        let mut stmt = conn.prepare("SELECT canonical_id, name, qualified_name, file_path, line_start, symbol_type FROM symbols JOIN files ON symbols.file_id = files.file_id")?;
        let rows = stmt.query_map([], |row| {
             Ok((
                row.get::<_, String>(0)?, // 🆕 canonical_id
                SnapshotSymbol {
                    name: row.get(1)?,
                    qualified_name: row.get(2)?,
                    file_path: row.get(3)?,
                    symbol_type: row.get(5)?,
                    line_start: row.get(4)?,
                    signature: None,
                    calls: vec![],
                }
             ))
        })?;

        for r in rows {
            if let Ok((id, sym)) = r {
                id_to_qname.insert(id.clone(), sym.qualified_name.clone());
                // Use canonical_id as stable key
                symbols_map.insert(id, sym);
            }
        }
    }
    
    // 2. Load Calls (hydrate symbols)
    {
        // 🆕 JOIN symbols 表获取 canonical_id
        let mut stmt = conn.prepare("SELECT s.canonical_id, c.callee_name FROM calls c JOIN symbols s ON c.caller_id = s.symbol_id")?;
        let rows = stmt.query_map([], |row| {
             Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
        })?;

        for r in rows {
            if let Ok((caller_canonical_id, callee_name)) = r {
                if let Some(sym) = symbols_map.get_mut(&caller_canonical_id) {
                    sym.calls.push(callee_name);
                }
            }
        }
    }
    
    let snapshot = Snapshot {
        timestamp: SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_secs(),
        symbols: symbols_map,
    };
    
    if let Some(out_path) = &args.output {
         let f = fs::File::create(out_path)?;
         serde_json::to_writer(f, &snapshot)?;
    } else {
         // Print to stdout? No, binary output usually silent unless error.
    }
    
    Ok(())
}

#[derive(Serialize)]
struct DiffResult {
    added: Vec<String>,
    removed: Vec<String>,
    modified: Vec<String>,
    details: HashMap<String, DiffDetail>,
}

#[derive(Serialize)]
struct DiffDetail {
    change_type: String, // "signature_changed", "calls_changed", "moved"
    diff_msg: String,
}

fn run_diff(args: &Args) -> anyhow::Result<()> {
    let base_path = args.base.as_ref().expect("Base snapshot required for diff");
    let target_path = args.target.as_ref().expect("Target snapshot required for diff");
    
    let base: Snapshot = serde_json::from_reader(fs::File::open(base_path)?)?;
    let target: Snapshot = serde_json::from_reader(fs::File::open(target_path)?)?;
    
    let mut added = vec![];
    let mut removed = vec![];
    let mut modified = vec![];
    let mut details = HashMap::new();
    
    // Check Removed
    for (k, _) in &base.symbols {
        if !target.symbols.contains_key(k) {
            removed.push(k.clone());
        }
    }
    
    // Check Added & Modified
    for (k, target_sym) in &target.symbols {
        if !base.symbols.contains_key(k) {
            added.push(k.clone());
        } else {
            let base_sym = base.symbols.get(k).unwrap();
            
            // Compare
            let mut diffs = vec![];
            
            if base_sym.file_path != target_sym.file_path {
                diffs.push(format!("Moved from {} to {}", base_sym.file_path, target_sym.file_path));
            }
            
            if base_sym.symbol_type != target_sym.symbol_type {
                diffs.push(format!("Type changed: {} -> {}", base_sym.symbol_type, target_sym.symbol_type));
            }
            
            // Check Calls
            let base_calls: HashSet<_> = base_sym.calls.iter().collect();
            let target_calls: HashSet<_> = target_sym.calls.iter().collect();
            
            let new_calls: Vec<_> = target_calls.difference(&base_calls).collect();
            let lost_calls: Vec<_> = base_calls.difference(&target_calls).collect();
            
            if !new_calls.is_empty() {
                diffs.push(format!("Added calls: {:?}", new_calls));
            }
            if !lost_calls.is_empty() {
                 diffs.push(format!("Removed calls: {:?}", lost_calls));
            }
            
            if !diffs.is_empty() {
                modified.push(k.clone());
                details.insert(k.clone(), DiffDetail {
                    change_type: "modified".into(),
                    diff_msg: diffs.join("; "),
                });
            }
        }
    }
    
    let res = DiffResult {
        added,
        removed,
        modified,
        details,
    };
    
    if let Some(out_path) = &args.output {
         let f = fs::File::create(out_path)?;
         serde_json::to_writer(f, &res)?;
    }
    
    Ok(())
}


// ============================================================================
// Structure Mode - 快速目录结构扫描 (No AST)
// ============================================================================

#[derive(Serialize)]
struct DirInfo {
    file_count: usize,
    files: Vec<String>,
}

#[derive(Serialize)]
struct StructureResult {
    status: String,
    total_files: usize,
    structure: HashMap<String, DirInfo>,
}

fn run_structure(args: &Args) -> anyhow::Result<()> {
    // 快速目录扫描，不做任何 AST 解析
    let project_path = Path::new(&args.project);
    
    // 构建目录遍历器
    let mut builder = WalkBuilder::new(&args.project);
    builder.hidden(false);
    builder.git_ignore(true);
    
    // 应用忽略目录过滤
    if let Some(ignores) = &args.ignore_dirs {
        let ignore_set: HashSet<String> = ignores.split(',').map(|s| s.trim().to_string()).collect();
        builder.filter_entry(move |entry| {
            if !entry.file_type().map(|f| f.is_dir()).unwrap_or(false) {
                return true;
            }
            !ignore_set.contains(entry.file_name().to_str().unwrap_or(""))
        });
    }
    
    // 应用扩展名过滤
    let allowed_exts: HashSet<String> = args.extensions.as_ref()
        .map(|s| s.split(',').map(|ext| ext.trim().trim_start_matches('.').to_string()).collect())
        .unwrap_or_default();
    
    // 收集文件，按目录分组
    let mut structure: HashMap<String, DirInfo> = HashMap::new();
    let mut total_files = 0;
    
    for entry in builder.build() {
        if let Ok(entry) = entry {
            if entry.file_type().map(|t| t.is_file()).unwrap_or(false) {
                let path = entry.path();
                
                // 扩展名过滤
                if !allowed_exts.is_empty() {
                    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
                    if !allowed_exts.contains(ext) {
                        continue;
                    }
                }
                
                // 计算相对路径
                let rel_path = path.strip_prefix(project_path).unwrap_or(path);
                let rel_str = rel_path.to_string_lossy().replace("\\", "/");
                
                // 提取目录和文件名
                let (dir, file_name) = if let Some(parent) = rel_path.parent() {
                    let parent_str = parent.to_string_lossy().replace("\\", "/");
                    let fname = rel_path.file_name().map(|n| n.to_string_lossy().to_string()).unwrap_or_default();
                    (parent_str, fname)
                } else {
                    ("".to_string(), rel_str.to_string())
                };
                
                // 添加到结构
                let dir_info = structure.entry(dir).or_insert(DirInfo {
                    file_count: 0,
                    files: vec![],
                });
                dir_info.file_count += 1;
                dir_info.files.push(file_name);
                total_files += 1;
            }
        }
    }
    
    // 输出结果
    let result = StructureResult {
        status: "success".to_string(),
        total_files,
        structure,
    };
    
    if let Some(out_path) = &args.output {
        let f = fs::File::create(out_path)?;
        serde_json::to_writer(f, &result)?;
    }
    
    Ok(())
}
