-- Remove the node status values added in migration 0017
DELETE FROM node_statuses WHERE name = 'PENDING';
DELETE FROM node_statuses WHERE name = 'STOPPING';
DELETE FROM node_statuses WHERE name = 'STARTING';
DELETE FROM node_statuses WHERE name = 'UPDATING';
