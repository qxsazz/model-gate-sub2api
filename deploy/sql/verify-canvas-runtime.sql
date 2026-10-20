BEGIN READ ONLY;
DO $$
BEGIN
 IF to_regclass('public.canvas_generation_tasks') IS NULL THEN
   RAISE EXCEPTION 'persistent canvas generation task table is missing';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='canvas_assets' AND column_name='file_deleted_at') THEN
   RAISE EXCEPTION 'canvas physical deletion retry column is missing';
 END IF;
END $$;
COMMIT;
