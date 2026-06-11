import {
  ForbiddenException,
  Injectable,
  Logger,
  NotFoundException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { InjectRepository } from '@nestjs/typeorm';
import { isUUID } from 'class-validator';
import { unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { Repository } from 'typeorm';

import { isAdmin, type AuthUser } from '../auth/auth-user';
import { Activity } from '../entities/activity.entity';
import { Attachment } from '../entities/attachment.entity';
import { Task, toTaskJson } from '../entities/task.entity';
import { EventsService } from '../events/events.service';
import { CreateTaskDto } from './dto/create-task.dto';
import { ListTasksDto } from './dto/list-tasks.dto';
import { UpdateTaskDto } from './dto/update-task.dto';

const taskNotFound = () =>
  new NotFoundException({ code: 'not_found', message: 'Task not found' });

const DUE_DATE_FMT = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
  timeZone: 'UTC',
});

@Injectable()
export class TasksService {
  private readonly logger = new Logger(TasksService.name);

  constructor(
    @InjectRepository(Task) private readonly tasks: Repository<Task>,
    @InjectRepository(Activity)
    private readonly activities: Repository<Activity>,
    @InjectRepository(Attachment)
    private readonly attachments: Repository<Attachment>,
    private readonly events: EventsService,
    private readonly config: ConfigService,
  ) {}

  /**
   * Fetches the task and enforces access rules: owners get full access,
   * admins may read any task, everyone else is told the task does not exist.
   */
  async loadTask(user: AuthUser, id: string, forWrite: boolean): Promise<Task> {
    if (!isUUID(id)) throw taskNotFound();
    const task = await this.tasks.findOne({
      where: { id },
      relations: { user: true },
    });
    if (!task) throw taskNotFound();
    if (task.userId === user.id) return task;
    if (isAdmin(user)) {
      if (!forWrite) return task;
      throw new ForbiddenException({
        code: 'forbidden',
        message: "Admins can view but not modify other users' tasks",
      });
    }
    // Hide the existence of other users' tasks.
    throw taskNotFound();
  }

  async recordActivity(
    taskId: string,
    userName: string,
    action: string,
    detail: string,
  ) {
    try {
      await this.activities.save(
        this.activities.create({ taskId, userName, action, detail }),
      );
    } catch (err) {
      // Activity logging must not fail the main operation.
      this.logger.warn(`record activity: ${String(err)}`);
    }
  }

  async create(user: AuthUser, dto: CreateTaskDto) {
    const task = this.tasks.create({
      userId: user.id,
      title: dto.title.trim(),
      description: dto.description ?? '',
      status: dto.status ?? 'todo',
      priority: dto.priority ?? 'medium',
      dueDate: dto.dueDate ? new Date(dto.dueDate) : null,
    });
    await this.tasks.save(task);
    await this.recordActivity(
      task.id,
      user.name,
      'created',
      `Created task "${task.title}"`,
    );

    const saved = await this.tasks.findOneOrFail({
      where: { id: task.id },
      relations: { user: true },
    });
    const json = toTaskJson(saved);
    this.events.publish(user.id, { type: 'task.created', payload: json });
    return { task: json };
  }

  async list(user: AuthUser, q: ListTasksDto) {
    const sortAliases: Record<string, string> = {
      created_at: 'created_at',
      createdAt: 'created_at',
      due_date: 'due_date',
      dueDate: 'due_date',
      priority: 'priority',
    };
    const sortBy = sortAliases[q.sortBy ?? 'created_at'];
    // Newest first by default; due dates read more naturally ascending.
    const order = (
      q.order ?? (sortBy === 'due_date' ? 'asc' : 'desc')
    ).toUpperCase() as 'ASC' | 'DESC';
    const page = q.page ?? 1;
    const limit = q.limit ?? 10;

    if (q.scope === 'all' && !isAdmin(user)) {
      throw new ForbiddenException({
        code: 'forbidden',
        message: "Only admins can view all users' tasks",
      });
    }

    const qb = this.tasks
      .createQueryBuilder('t')
      .innerJoinAndSelect('t.user', 'u');
    if (q.scope !== 'all') {
      qb.andWhere('t.user_id = :uid', { uid: user.id });
    }
    if (q.status) {
      qb.andWhere('t.status = :status', { status: q.status });
    }
    const search = q.search?.trim();
    if (search) {
      const escaped = search.replace(/[\\%_]/g, (m) => `\\${m}`);
      qb.andWhere(`t.title ILIKE :search ESCAPE '\\'`, {
        search: `%${escaped}%`,
      });
    }

    switch (sortBy) {
      case 'due_date':
        qb.orderBy('t.due_date', order, 'NULLS LAST').addOrderBy(
          't.created_at',
          'DESC',
        );
        break;
      case 'priority':
        qb.orderBy(
          `CASE t.priority WHEN 'high' THEN 3 WHEN 'medium' THEN 2 ELSE 1 END`,
          order,
        ).addOrderBy('t.created_at', 'DESC');
        break;
      default:
        qb.orderBy('t.created_at', order);
    }

    qb.skip((page - 1) * limit).take(limit);
    const [items, total] = await qb.getManyAndCount();
    return {
      tasks: items.map(toTaskJson),
      meta: {
        page,
        limit,
        total,
        totalPages: total > 0 ? Math.ceil(total / limit) : 0,
      },
    };
  }

  async get(user: AuthUser, id: string) {
    const task = await this.loadTask(user, id, false);
    return { task: toTaskJson(task) };
  }

  async update(user: AuthUser, id: string, dto: UpdateTaskDto) {
    const task = await this.loadTask(user, id, true);

    const changes: string[] = [];
    if (dto.title !== undefined) {
      const title = dto.title.trim();
      if (title !== task.title) {
        changes.push(`renamed to "${title}"`);
        task.title = title;
      }
    }
    if (dto.description !== undefined && dto.description !== task.description) {
      task.description = dto.description;
      changes.push('updated the description');
    }
    if (dto.status !== undefined && dto.status !== task.status) {
      changes.push(`moved from ${task.status} to ${dto.status}`);
      task.status = dto.status;
    }
    if (dto.priority !== undefined && dto.priority !== task.priority) {
      changes.push(`changed priority from ${task.priority} to ${dto.priority}`);
      task.priority = dto.priority;
    }
    if (dto.dueDate !== undefined) {
      if (dto.dueDate === '') {
        if (task.dueDate) {
          task.dueDate = null;
          changes.push('cleared the due date');
        }
      } else {
        const due = new Date(dto.dueDate);
        if (!task.dueDate || task.dueDate.getTime() !== due.getTime()) {
          task.dueDate = due;
          changes.push(`set the due date to ${DUE_DATE_FMT.format(due)}`);
        }
      }
    }

    await this.tasks.save(task);
    if (changes.length > 0) {
      await this.recordActivity(
        task.id,
        user.name,
        'updated',
        changes.join('; '),
      );
    }
    const json = toTaskJson(task);
    this.events.publish(task.userId, { type: 'task.updated', payload: json });
    return { task: json };
  }

  async remove(user: AuthUser, id: string) {
    const task = await this.loadTask(user, id, true);
    const taskId = task.id;
    const ownerId = task.userId;
    const files = await this.attachments.find({ where: { taskId } });

    await this.tasks.remove(task); // activities/attachments cascade in the DB

    const uploadDir = this.config.get<string>('uploadDir') ?? './uploads';
    for (const file of files) {
      await unlink(join(uploadDir, file.storedName)).catch(() => undefined);
    }
    this.events.publish(ownerId, {
      type: 'task.deleted',
      payload: { id: taskId },
    });
  }

  async activity(user: AuthUser, id: string) {
    const task = await this.loadTask(user, id, false);
    const items = await this.activities.find({
      where: { taskId: task.id },
      order: { createdAt: 'DESC' },
    });
    return {
      activity: items.map((a) => ({
        id: a.id,
        taskId: a.taskId,
        userName: a.userName,
        action: a.action,
        detail: a.detail,
        createdAt: a.createdAt,
      })),
    };
  }
}
