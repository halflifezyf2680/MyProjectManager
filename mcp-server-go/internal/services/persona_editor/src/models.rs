use serde::{Deserialize, Serialize};

/// 人格配置（V2.0 Skin-Only 格式）
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct PersonaConfig {
    pub name: String,
    pub display_name: String,
    #[serde(default)]
    pub avatar: String,
    #[serde(default)]
    pub aliases: Vec<String>,
    #[serde(default)]
    pub style_must: Vec<String>,
    #[serde(default)]
    pub style_signature: Vec<String>,
    #[serde(default)]
    pub style_taboo: Vec<String>,
}

#[derive(Serialize, Deserialize)]
pub struct PersonaLibrary {
    pub personas: Vec<PersonaConfig>,
}
