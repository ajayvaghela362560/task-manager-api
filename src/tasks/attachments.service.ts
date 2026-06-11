import {
  BadRequestException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { isUUID } from 'class-validator';
import type { Response } from 'express';
import { unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { Repository } from 'typeorm';

import { isAdmin, type AuthUser } from '../auth/auth-user';
import { Attachment, toAttachmentJson } from '../entities/attachment.entity';
import { Task, toTaskJson } from '../entities/task.entity';
import { EventsService } from '../events/events.service';
import { TasksService } from './tasks.service';
import { UPLOAD_DIR } from './upload.config';

const attachmentNotFound = () =>
  new NotFoundException({ code: 'not_found', message: 'Attachment not found' });

@Injectable()
export class AttachmentsService {
  constructor(
    @InjectRepository(Attachment)
    private readonly attachments: Repository<Attachment>,
    private readonly tasksService: TasksService,
    private readonly events: EventsService,
  ) {}

  async listForTask(user: AuthUser, taskId: string) {
    const task = await this.tasksService.loadTask(user, taskId, false);
    const items = await this.attachments.find({
      where: { taskId: task.id },
      order: { createdAt: 'ASC' },
    });
    return { attachments: items.map(toAttachmentJson) };
  }

  async upload(user: AuthUser, taskId: string, file?: Express.Multer.File) {
    if (!file) {
      throw new BadRequestException({
        code: 'bad_request',
        message: 'Missing "file" form field',
      });
    }
    // Multer has already written the file to disk; remove it again if the
    // task does not exist or the caller may not modify it.
    let task: Task;
    try {
      task = await this.tasksService.loadTask(user, taskId, true);
    } catch (err) {
      await unlink(file.path).catch(() => undefined);
      throw err;
    }

    const attachment = this.attachments.create({
      taskId: task.id,
      fileName: file.originalname,
      storedName: file.filename,
      contentType: file.mimetype || 'application/octet-stream',
      sizeBytes: file.size,
    });
    await this.attachments.save(attachment);
    await this.tasksService.recordActivity(
      task.id,
      user.name,
      'attachment_added',
      `Attached file "${attachment.fileName}"`,
    );
    this.events.publish(task.userId, {
      type: 'task.updated',
      payload: toTaskJson(task),
    });
    return { attachment: toAttachmentJson(attachment) };
  }

  /** Resolves the attachment and checks access to its parent task. */
  private async loadAttachment(user: AuthUser, id: string, forWrite: boolean) {
    if (!isUUID(id)) throw attachmentNotFound();
    const attachment = await this.attachments.findOne({
      where: { id },
      relations: { task: true },
    });
    if (!attachment) throw attachmentNotFound();
    const task = attachment.task;
    if (task.userId === user.id || (!forWrite && isAdmin(user))) {
      return { attachment, task };
    }
    throw attachmentNotFound();
  }

  async download(user: AuthUser, id: string, res: Response) {
    const { attachment } = await this.loadAttachment(user, id, false);
    res.setHeader('Content-Type', attachment.contentType);
    res.download(join(UPLOAD_DIR, attachment.storedName), attachment.fileName);
  }

  async remove(user: AuthUser, id: string) {
    const { attachment, task } = await this.loadAttachment(user, id, true);
    const { storedName, fileName } = attachment;

    await this.attachments.remove(attachment);
    await unlink(join(UPLOAD_DIR, storedName)).catch(() => undefined);
    await this.tasksService.recordActivity(
      task.id,
      user.name,
      'attachment_removed',
      `Removed file "${fileName}"`,
    );
    this.events.publish(task.userId, {
      type: 'task.updated',
      payload: toTaskJson(task),
    });
  }
}
