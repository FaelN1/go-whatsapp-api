-- Migration: Create analytics tables for message tracking
-- Version: 001
-- Description: Creates tables for tracking sent messages, views, and reactions
-- Database: PostgreSQL

-- Table: message_tracking
-- Stores all sent messages that should be tracked
CREATE TABLE IF NOT EXISTS message_tracking (
    id VARCHAR(255) PRIMARY KEY,
    instance_id VARCHAR(255) NOT NULL,
    message_id VARCHAR(255) NOT NULL,
    remote_jid VARCHAR(255) NOT NULL,
    community_jid VARCHAR(255),
    message_type VARCHAR(50) NOT NULL,
    content TEXT,
    media_url TEXT,
    caption TEXT,
    sent_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for message_tracking
CREATE INDEX IF NOT EXISTS idx_message_tracking_instance ON message_tracking(instance_id);
CREATE INDEX IF NOT EXISTS idx_message_tracking_message_id ON message_tracking(message_id);
CREATE INDEX IF NOT EXISTS idx_message_tracking_remote_jid ON message_tracking(remote_jid);
CREATE INDEX IF NOT EXISTS idx_message_tracking_community_jid ON message_tracking(community_jid);
CREATE INDEX IF NOT EXISTS idx_message_tracking_sent_at ON message_tracking(sent_at DESC);

-- Table: message_views
-- Stores who viewed each tracked message
CREATE TABLE IF NOT EXISTS message_views (
    id VARCHAR(255) PRIMARY KEY,
    message_track_id VARCHAR(255) NOT NULL,
    viewer_jid VARCHAR(255) NOT NULL,
    viewer_name VARCHAR(255),
    viewed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_message_views_tracking FOREIGN KEY (message_track_id) 
        REFERENCES message_tracking(id) ON DELETE CASCADE,
    CONSTRAINT unique_message_viewer UNIQUE (message_track_id, viewer_jid)
);

-- Indexes for message_views
CREATE INDEX IF NOT EXISTS idx_message_views_track_id ON message_views(message_track_id);
CREATE INDEX IF NOT EXISTS idx_message_views_viewer_jid ON message_views(viewer_jid);
CREATE INDEX IF NOT EXISTS idx_message_views_viewed_at ON message_views(viewed_at DESC);

-- Table: message_reactions
-- Stores reactions to tracked messages
CREATE TABLE IF NOT EXISTS message_reactions (
    id VARCHAR(255) PRIMARY KEY,
    message_track_id VARCHAR(255) NOT NULL,
    reactor_jid VARCHAR(255) NOT NULL,
    reactor_name VARCHAR(255),
    reaction VARCHAR(50) NOT NULL,
    reacted_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_message_reactions_tracking FOREIGN KEY (message_track_id) 
        REFERENCES message_tracking(id) ON DELETE CASCADE,
    CONSTRAINT unique_message_reactor UNIQUE (message_track_id, reactor_jid)
);

-- Indexes for message_reactions
CREATE INDEX IF NOT EXISTS idx_message_reactions_track_id ON message_reactions(message_track_id);
CREATE INDEX IF NOT EXISTS idx_message_reactions_reactor_jid ON message_reactions(reactor_jid);
CREATE INDEX IF NOT EXISTS idx_message_reactions_reacted_at ON message_reactions(reacted_at DESC);

-- Create view for aggregated metrics
CREATE OR REPLACE VIEW message_metrics_summary AS
SELECT 
    mt.id as tracking_id,
    mt.instance_id,
    mt.message_id,
    mt.remote_jid,
    mt.community_jid,
    mt.message_type,
    mt.sent_at,
    COUNT(DISTINCT mv.id) as view_count,
    COUNT(DISTINCT mr.id) as reaction_count,
    STRING_AGG(DISTINCT mr.reaction, ',' ORDER BY mr.reaction) as reactions
FROM message_tracking mt
LEFT JOIN message_views mv ON mt.id = mv.message_track_id
LEFT JOIN message_reactions mr ON mt.id = mr.message_track_id
GROUP BY mt.id, mt.instance_id, mt.message_id, mt.remote_jid, 
         mt.community_jid, mt.message_type, mt.sent_at;
