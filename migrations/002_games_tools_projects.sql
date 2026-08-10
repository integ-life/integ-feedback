INSERT INTO projects(slug,name) VALUES
  ('games','Integ Games'),
  ('tools','Integ Tools')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO project_keys(project_id,key_hash,label)
SELECT id,encode(digest('pk_games_web_v1_7b4e1a','sha256'),'hex'),'games web v1'
FROM projects WHERE slug='games'
ON CONFLICT (key_hash) DO NOTHING;

INSERT INTO project_keys(project_id,key_hash,label)
SELECT id,encode(digest('pk_tools_web_v1_9c2f6d','sha256'),'hex'),'tools web v1'
FROM projects WHERE slug='tools'
ON CONFLICT (key_hash) DO NOTHING;
