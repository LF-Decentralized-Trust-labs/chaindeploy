DELETE FROM node_statuses WHERE name IN ('creating', 'genesis_block_created', 'running', 'stopped', 'error', 'proposed', 'imported');
DELETE FROM blockchain_platforms WHERE name IN ('fabric', 'besu');
