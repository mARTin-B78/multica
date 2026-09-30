ALTER TABLE litellm_connection
ADD COLUMN IF NOT EXISTS management_api_key_encrypted TEXT;
