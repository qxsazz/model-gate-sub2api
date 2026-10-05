-- Fictional transaction fixture; never apply as production data.
CREATE TEMP TABLE users(id bigint PRIMARY KEY,role text,status text,deleted_at timestamptz,balance numeric);
INSERT INTO users VALUES(1,'admin','active',NULL,10),(2,'user','active',NULL,20);
CREATE TEMP TABLE achievement_config(id boolean,payload jsonb);
INSERT INTO achievement_config VALUES(true,'{"cash_enabled":false,"milestone_cash_enabled":false}');
CREATE TEMP TABLE achievement_growth(user_id bigint PRIMARY KEY,tokens bigint,last_paid_at timestamptz,last_balance_paid_at timestamptz);
CREATE TEMP TABLE usage_logs(id bigint,user_id bigint,api_key_id bigint,request_id text,model text,billing_mode text,actual_cost numeric,input_tokens int,output_tokens int,cache_creation_tokens int,cache_read_tokens int,image_count int,video_count int,image_input_tokens int,image_output_tokens int,created_at timestamptz);
CREATE TEMP TABLE usage_billing_dedup(request_id text,api_key_id bigint,request_fingerprint text);
CREATE TEMP TABLE usage_billing_dedup_archive(request_id text,api_key_id bigint,request_fingerprint text);
INSERT INTO usage_logs VALUES(1,2,20,'paid','gpt-test','token',.01,100,50,25,125,0,0,0,0,now()),(2,2,20,'media','gpt-image-2','token',.01,100,0,0,0,0,0,0,0,now()),(3,2,20,'free','gpt-test','token',0,100,0,0,0,0,0,0,0,now()),(4,2,20,'unconfirmed','gpt-test','token',.01,100,0,0,0,0,0,0,0,now());
INSERT INTO usage_billing_dedup VALUES('paid',20,'fixture'),('media',20,'fixture'),('free',20,'fixture');
