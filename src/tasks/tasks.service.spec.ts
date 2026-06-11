import { ForbiddenException, NotFoundException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { randomUUID } from 'node:crypto';
import type { Repository } from 'typeorm';

import type { AuthUser } from '../auth/auth-user';
import { Activity } from '../entities/activity.entity';
import { Attachment } from '../entities/attachment.entity';
import { Task } from '../entities/task.entity';
import { User } from '../entities/user.entity';
import { EventsService } from '../events/events.service';
import { TasksService } from './tasks.service';

const owner: AuthUser = { id: randomUUID(), name: 'Ada', role: 'user' };
const stranger: AuthUser = { id: randomUUID(), name: 'Bob', role: 'user' };
const admin: AuthUser = { id: randomUUID(), name: 'Root', role: 'admin' };

function fakeTaskRepo() {
  const rows = new Map<string, Task>();
  const users = new Map<string, User>([
    [
      owner.id,
      Object.assign(new User(), { id: owner.id, email: 'ada@example.com' }),
    ],
  ]);
  const repo = {
    create: (data: Partial<Task>) => Object.assign(new Task(), data),
    save: (t: Task) => {
      if (!t.id) {
        t.id = randomUUID();
        t.createdAt = new Date();
      }
      t.updatedAt = new Date();
      rows.set(t.id, t);
      return Promise.resolve(t);
    },
    findOne: ({ where }: { where: { id: string } }) => {
      const t = rows.get(where.id) ?? null;
      if (t) t.user = users.get(t.userId) ?? new User();
      return Promise.resolve(t);
    },
    findOneOrFail: async (opts: { where: { id: string } }) => {
      const t = await repo.findOne(opts);
      if (!t) throw new Error('not found');
      return t;
    },
    remove: (t: Task) => {
      rows.delete(t.id);
      return Promise.resolve(t);
    },
  };
  return repo as unknown as Repository<Task>;
}

function fakeActivityRepo() {
  const rows: Activity[] = [];
  const repo = {
    create: (data: Partial<Activity>) => Object.assign(new Activity(), data),
    save: (a: Activity) => {
      a.id = randomUUID();
      a.createdAt = new Date();
      rows.push(a);
      return Promise.resolve(a);
    },
    find: ({ where }: { where: { taskId: string } }) =>
      Promise.resolve(rows.filter((a) => a.taskId === where.taskId).reverse()),
  };
  return { repo: repo as unknown as Repository<Activity>, rows };
}

function fakeAttachmentRepo() {
  return {
    find: () => Promise.resolve([]),
  } as unknown as Repository<Attachment>;
}

function makeService() {
  const activities = fakeActivityRepo();
  const service = new TasksService(
    fakeTaskRepo(),
    activities.repo,
    fakeAttachmentRepo(),
    new EventsService(),
    new ConfigService({ uploadDir: '/tmp/test-uploads' }),
  );
  return { service, activityRows: activities.rows };
}

describe('TasksService', () => {
  it('applies defaults on create', async () => {
    const { service } = makeService();
    const { task } = await service.create(owner, { title: '  Ship it  ' });
    expect(task.title).toBe('Ship it'); // trimmed
    expect(task.status).toBe('todo');
    expect(task.priority).toBe('medium');
    expect(task.dueDate).toBeNull();
    expect(task.ownerEmail).toBe('ada@example.com');
  });

  it('hides other users’ tasks (404) and lets owners read them', async () => {
    const { service } = makeService();
    const { task } = await service.create(owner, { title: 'Private' });

    await expect(service.get(owner, task.id)).resolves.toBeDefined();
    await expect(service.get(stranger, task.id)).rejects.toThrow(
      NotFoundException,
    );
    await expect(
      service.update(stranger, task.id, { status: 'done' }),
    ).rejects.toThrow(NotFoundException);
    await expect(service.remove(stranger, task.id)).rejects.toThrow(
      NotFoundException,
    );
  });

  it('lets admins read but not modify other users’ tasks', async () => {
    const { service } = makeService();
    const { task } = await service.create(owner, { title: 'Private' });

    await expect(service.get(admin, task.id)).resolves.toBeDefined();
    await expect(
      service.update(admin, task.id, { status: 'done' }),
    ).rejects.toThrow(ForbiddenException);
    await expect(service.remove(admin, task.id)).rejects.toThrow(
      ForbiddenException,
    );
  });

  it('blocks the all-users scope for non-admins', async () => {
    const { service } = makeService();
    await expect(service.list(stranger, { scope: 'all' })).rejects.toThrow(
      ForbiddenException,
    );
  });

  it('applies partial updates and records the change history', async () => {
    const { service, activityRows } = makeService();
    const { task } = await service.create(owner, {
      title: 'Draft',
      priority: 'low',
    });

    const { task: updated } = await service.update(owner, task.id, {
      status: 'done',
      dueDate: '2026-06-20T00:00:00Z',
    });
    expect(updated.status).toBe('done');
    expect(updated.priority).toBe('low'); // untouched
    expect(updated.dueDate).toBe('2026-06-20T00:00:00.000Z');

    const { task: cleared } = await service.update(owner, task.id, {
      dueDate: '',
    });
    expect(cleared.dueDate).toBeNull();

    const actions = activityRows.map((a) => a.action);
    expect(actions).toEqual(['created', 'updated', 'updated']);
    expect(activityRows[1].detail).toContain('moved from todo to done');
  });

  it('returns 404 for malformed and deleted task ids', async () => {
    const { service } = makeService();
    await expect(service.get(owner, 'not-a-uuid')).rejects.toThrow(
      NotFoundException,
    );

    const { task } = await service.create(owner, { title: 'Short-lived' });
    await service.remove(owner, task.id);
    await expect(service.get(owner, task.id)).rejects.toThrow(
      NotFoundException,
    );
  });
});
