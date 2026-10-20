-- Canvas inputs may contain reference images even in small JSON requests.
-- Keep operation metadata, but do not retain a second media copy in general audit.
UPDATE audit_logs
SET request_body = '<private canvas payload omitted>'
WHERE path LIKE '/api/v1/canvas/%'
  AND method IN ('POST','PUT','PATCH','DELETE')
  AND request_body <> '<private canvas payload omitted>';
