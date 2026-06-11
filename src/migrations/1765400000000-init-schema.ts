import { MigrationInterface, QueryRunner } from 'typeorm';

/**
 * Initial schema. Uses IF NOT EXISTS throughout so it also applies cleanly
 * to databases created by earlier versions of this app.
 */
export class InitSchema1765400000000 implements MigrationInterface {
  name = 'InitSchema1765400000000';

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      CREATE TABLE IF NOT EXISTS users (
          id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          name          TEXT NOT NULL,
          email         TEXT NOT NULL UNIQUE,
          password_hash TEXT NOT NULL,
          role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
          created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
      );

      CREATE TABLE IF NOT EXISTS tasks (
          id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
          title       TEXT NOT NULL,
          description TEXT NOT NULL DEFAULT '',
          status      TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
          priority    TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
          due_date    TIMESTAMPTZ,
          created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
      );

      CREATE INDEX IF NOT EXISTS idx_tasks_user_status ON tasks (user_id, status);
      CREATE INDEX IF NOT EXISTS idx_tasks_due_date    ON tasks (due_date);
      CREATE INDEX IF NOT EXISTS idx_tasks_created_at  ON tasks (created_at);

      CREATE TABLE IF NOT EXISTS task_activities (
          id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
          user_name  TEXT NOT NULL DEFAULT '',
          action     TEXT NOT NULL,
          detail     TEXT NOT NULL DEFAULT '',
          created_at TIMESTAMPTZ NOT NULL DEFAULT now()
      );

      CREATE INDEX IF NOT EXISTS idx_activities_task ON task_activities (task_id, created_at DESC);

      CREATE TABLE IF NOT EXISTS attachments (
          id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          task_id      UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
          file_name    TEXT NOT NULL,
          stored_name  TEXT NOT NULL,
          content_type TEXT NOT NULL,
          size_bytes   BIGINT NOT NULL,
          created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
      );

      CREATE INDEX IF NOT EXISTS idx_attachments_task ON attachments (task_id);
    `);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      DROP TABLE IF EXISTS attachments;
      DROP TABLE IF EXISTS task_activities;
      DROP TABLE IF EXISTS tasks;
      DROP TABLE IF EXISTS users;
    `);
  }
}
