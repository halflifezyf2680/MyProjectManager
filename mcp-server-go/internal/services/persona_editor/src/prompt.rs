use serde::{Deserialize, Serialize};
use rusqlite::{Connection, params};
use crate::discovery::resolve_db_path;

#[derive(Serialize, Deserialize, Debug)]
pub struct PromptSnippet {
    #[serde(default)]
    pub id: Option<i64>,
    pub title: String,
    pub content: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default)]
    pub use_count: i64,
    #[serde(default)]
    pub is_favorite: bool,
}

// 初始化表结构
fn init_db(conn: &Connection) -> rusqlite::Result<()> {
    conn.execute_batch(
        r#"
        CREATE TABLE IF NOT EXISTS prompt_snippets (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            category TEXT DEFAULT 'general',
            is_favorite INTEGER DEFAULT 0,
            use_count INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS snippet_tags (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            color TEXT DEFAULT '#4285F4',
            use_count INTEGER DEFAULT 0
        );

        CREATE TABLE IF NOT EXISTS snippet_tag_relations (
            snippet_id INTEGER,
            tag_id INTEGER,
            PRIMARY KEY (snippet_id, tag_id),
            FOREIGN KEY (snippet_id) REFERENCES prompt_snippets(id) ON DELETE CASCADE,
            FOREIGN KEY (tag_id) REFERENCES snippet_tags(id) ON DELETE CASCADE
        );
        "#,
    )
}

#[tauri::command]
pub fn list_prompts(project_path: String) -> Result<Vec<PromptSnippet>, String> {
    let (db_path, valid) = resolve_db_path(project_path);
    if !valid {
        return Err("Invalid DB Path".to_string());
    }

    let conn = Connection::open(&db_path).map_err(|e| e.to_string())?;
    init_db(&conn).map_err(|e| e.to_string())?;

    let mut stmt = conn.prepare(
        "SELECT id, title, content, category, use_count, is_favorite FROM prompt_snippets ORDER BY updated_at DESC"
    ).map_err(|e| e.to_string())?;

    let iter = stmt.query_map([], |row| {
        let id: i64 = row.get(0)?;
        Ok(PromptSnippet {
            id: Some(id),
            title: row.get(1)?,
            content: row.get(2)?,
            category: row.get(3)?,
            use_count: row.get(4)?,
            is_favorite: row.get(5)?,
            tags: vec![], // Fill later
        })
    }).map_err(|e| e.to_string())?;

    let mut prompts = Vec::new();
    for p in iter {
        if let Ok(mut prompt) = p {
            // Load tags
            let tags_res: Result<Vec<String>, _> = conn.prepare(
                "SELECT t.name FROM snippet_tags t 
                 JOIN snippet_tag_relations r ON t.id = r.tag_id 
                 WHERE r.snippet_id = ?"
            ).and_then(|mut s| {
                 let rows = s.query_map([prompt.id], |r| r.get(0))?;
                 rows.collect()
            });
            
            if let Ok(tags) = tags_res {
                prompt.tags = tags;
            }
            prompts.push(prompt);
        }
    }

    Ok(prompts)
}

#[tauri::command]
pub fn save_prompt_snippet(project_path: String, snippet: PromptSnippet) -> Result<i64, String> {
    let (db_path, valid) = resolve_db_path(project_path.clone());
    if !valid {
        return Err("Invalid DB Path".to_string());
    }

    // Connect with Foreign Keys enabled
    let conn = Connection::open(&db_path).map_err(|e| e.to_string())?;
    conn.pragma_update(None, "foreign_keys", "ON").map_err(|e| e.to_string())?;
    init_db(&conn).map_err(|e| e.to_string())?; // Ensure tables exist

    // Update or Insert
    let final_id = if let Some(id) = snippet.id {
        if id > 0 {
            // UPDATE
            conn.execute(
                "UPDATE prompt_snippets SET title = ?1, content = ?2, category = ?3, updated_at = CURRENT_TIMESTAMP WHERE id = ?4",
                params![snippet.title, snippet.content, snippet.category, id],
            ).map_err(|e| e.to_string())?;
            
            // Update tags: remove all relations and re-add (simple strategy)
            conn.execute("DELETE FROM snippet_tag_relations WHERE snippet_id = ?", [id]).map_err(|e| e.to_string())?;
            id
        } else {
            // INSERT (id is Some(0) or Some(negative))
            conn.execute(
                "INSERT INTO prompt_snippets (title, content, category) VALUES (?1, ?2, ?3)",
                params![snippet.title, snippet.content, snippet.category],
            ).map_err(|e| e.to_string())?;
            conn.last_insert_rowid()
        }
    } else {
        // INSERT (id is None)
        conn.execute(
            "INSERT INTO prompt_snippets (title, content, category) VALUES (?1, ?2, ?3)",
            params![snippet.title, snippet.content, snippet.category],
        ).map_err(|e| e.to_string())?;
        conn.last_insert_rowid()
    };

    // Handle Tags
    for tag in &snippet.tags {
        let tag_trim = tag.trim();
        if tag_trim.is_empty() { continue; }
        
        // Find or create tag
        let tag_id: i64 = match conn.query_row("SELECT id FROM snippet_tags WHERE name = ?", [tag_trim], |r| r.get(0)) {
            Ok(id) => id,
            Err(_) => {
                conn.execute("INSERT INTO snippet_tags (name) VALUES (?)", [tag_trim]).map_err(|e| e.to_string())?;
                conn.last_insert_rowid()
            }
        };
        
        // Link
        conn.execute(
            "INSERT OR IGNORE INTO snippet_tag_relations (snippet_id, tag_id) VALUES (?, ?)", 
            params![final_id, tag_id]
        ).map_err(|e| e.to_string())?;
    }

    Ok(final_id)
}

#[tauri::command]
pub fn delete_prompt_snippet(project_path: String, id: i64) -> Result<(), String> {
    let (db_path, valid) = resolve_db_path(project_path);
    if !valid { return Err("Invalid DB Path".to_string()); }

    let conn = Connection::open(&db_path).map_err(|e| e.to_string())?;
    conn.execute("DELETE FROM prompt_snippets WHERE id = ?", [id]).map_err(|e| e.to_string())?;
    Ok(())
}
