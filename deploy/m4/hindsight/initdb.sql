-- M4 Hindsight schema — the minimal POC contract
-- Source: version 1/installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md §1a
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE memory_entries (
  task_id   text,
  agent_ref text,
  scope     text,              -- 'session' | 'cross-task' (the scoping rule from the plan)
  body      jsonb,
  embedding vector(1536),     -- dimension per provider (1536 = OpenAI text-embedding-3-small);
                              -- changing provider/dimension forces a table rebuild — record it
  created   timestamptz default now()
);
