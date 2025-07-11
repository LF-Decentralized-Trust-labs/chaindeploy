-- Add endorsement policy field to chaincode_projects table
ALTER TABLE chaincode_projects ADD COLUMN endorsement_policy TEXT; 