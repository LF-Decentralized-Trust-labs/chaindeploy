-- Squashed migrations combining all schema changes

-- Create chaincode_projects table
CREATE TABLE chaincode_projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    boilerplate TEXT, -- Boilerplate template used for scaffolding
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    slug TEXT NOT NULL UNIQUE,
    container_id TEXT,
    container_name TEXT,
    status TEXT,
    last_started_at DATETIME,
    last_stopped_at DATETIME,
    container_port INTEGER,
    network_id INTEGER REFERENCES networks(id)
);

-- Create conversations table
CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES chaincode_projects(id)
);

-- Create messages table
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    parent_id INTEGER,
    sender TEXT NOT NULL, -- 'user' or 'assistant'
    content TEXT NOT NULL,
    enhanced_content TEXT, -- Enhanced version of the content for AI interaction
    tool_arguments TEXT, -- JSON string containing tool arguments
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    FOREIGN KEY (parent_id) REFERENCES messages(id)
);

-- Create tool calls table
CREATE TABLE IF NOT EXISTS tool_calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    tool_name TEXT NOT NULL,
    arguments TEXT NOT NULL, -- JSON string
    result TEXT, -- JSON string
    error TEXT, -- nullable error string
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (message_id) REFERENCES messages(id)
); 