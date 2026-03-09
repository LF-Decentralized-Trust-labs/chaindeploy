-- Add missing node status values for proper lifecycle management
INSERT INTO node_statuses (name) VALUES ('PENDING');
INSERT INTO node_statuses (name) VALUES ('STOPPING');
INSERT INTO node_statuses (name) VALUES ('STARTING');
INSERT INTO node_statuses (name) VALUES ('UPDATING');
