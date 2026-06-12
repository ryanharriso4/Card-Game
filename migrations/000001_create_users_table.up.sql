CREATE EXTENSION IF NOT EXISTS citext;


CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    email text NOT NULL,
    username text NOT NULL, 
    oidc_sub varchar(255) UNIQUE NOT NULL
);