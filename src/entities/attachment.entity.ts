import {
  Column,
  CreateDateColumn,
  Entity,
  JoinColumn,
  ManyToOne,
  PrimaryGeneratedColumn,
} from 'typeorm';

import { Task } from './task.entity';

@Entity('attachments')
export class Attachment {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ name: 'task_id', type: 'uuid' })
  taskId: string;

  @ManyToOne(() => Task, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'task_id' })
  task: Task;

  @Column({ name: 'file_name' })
  fileName: string;

  @Column({ name: 'stored_name' })
  storedName: string;

  @Column({ name: 'content_type' })
  contentType: string;

  // Postgres bigint comes back as a string; converted in toAttachmentJson.
  @Column({ name: 'size_bytes', type: 'bigint' })
  sizeBytes: string | number;

  @CreateDateColumn({ name: 'created_at', type: 'timestamptz' })
  createdAt: Date;
}

export function toAttachmentJson(a: Attachment) {
  return {
    id: a.id,
    taskId: a.taskId,
    fileName: a.fileName,
    contentType: a.contentType,
    sizeBytes: Number(a.sizeBytes),
    createdAt: a.createdAt,
  };
}
