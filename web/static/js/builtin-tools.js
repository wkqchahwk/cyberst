/**
 * English note.
 * English note.
 * 
 * English note.
 */

// English note.
const BuiltinTools = {
    // English note.
    RECORD_VULNERABILITY: 'record_vulnerability',
    
    // English note.
    LIST_KNOWLEDGE_RISK_TYPES: 'list_knowledge_risk_types',
    SEARCH_KNOWLEDGE_BASE: 'search_knowledge_base'
};

// English note.
function isBuiltinTool(toolName) {
    return Object.values(BuiltinTools).includes(toolName);
}

// English note.
function getAllBuiltinTools() {
    return Object.values(BuiltinTools);
}

