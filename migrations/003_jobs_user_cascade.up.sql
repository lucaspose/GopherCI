ALTER TABLE jobs DROP CONSTRAINT jobs_user_id_fkey;

ALTER TABLE jobs ADD CONSTRAINT jobs_user_id_fkey 
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;