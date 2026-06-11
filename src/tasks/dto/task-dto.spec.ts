import { plainToInstance } from 'class-transformer';
import { validate } from 'class-validator';

import { LoginDto } from '../../auth/dto/login.dto';
import { SignupDto } from '../../auth/dto/signup.dto';
import { CreateTaskDto } from './create-task.dto';
import { ListTasksDto } from './list-tasks.dto';
import { UpdateTaskDto } from './update-task.dto';

type Ctor<T> = new () => T;

async function failedFields<T extends object>(
  cls: Ctor<T>,
  plain: object,
): Promise<string[]> {
  const errors = await validate(plainToInstance(cls, plain));
  return errors.map((e) => e.property);
}

describe('CreateTaskDto', () => {
  it('accepts a fully valid payload', async () => {
    const fields = await failedFields(CreateTaskDto, {
      title: 'Write report',
      description: 'Quarterly numbers',
      status: 'in_progress',
      priority: 'high',
      dueDate: '2026-06-15T17:00:00Z',
    });
    expect(fields).toEqual([]);
  });

  it('accepts a title alone', async () => {
    expect(
      await failedFields(CreateTaskDto, { title: 'Just a title' }),
    ).toEqual([]);
  });

  it('requires a non-blank title', async () => {
    expect(await failedFields(CreateTaskDto, {})).toContain('title');
    expect(await failedFields(CreateTaskDto, { title: '   ' })).toContain(
      'title',
    );
  });

  it('rejects an overlong title', async () => {
    expect(
      await failedFields(CreateTaskDto, { title: 'x'.repeat(201) }),
    ).toContain('title');
  });

  it('rejects invalid status and priority values', async () => {
    expect(
      await failedFields(CreateTaskDto, { title: 'ok', status: 'archived' }),
    ).toContain('status');
    expect(
      await failedFields(CreateTaskDto, { title: 'ok', priority: 'urgent' }),
    ).toContain('priority');
  });

  it('rejects a malformed due date but allows an empty one', async () => {
    expect(
      await failedFields(CreateTaskDto, { title: 'ok', dueDate: 'tomorrow' }),
    ).toContain('dueDate');
    expect(
      await failedFields(CreateTaskDto, { title: 'ok', dueDate: '' }),
    ).toEqual([]);
  });
});

describe('UpdateTaskDto', () => {
  it('accepts an empty patch (no fields)', async () => {
    expect(await failedFields(UpdateTaskDto, {})).toEqual([]);
  });

  it('cannot blank the title', async () => {
    expect(await failedFields(UpdateTaskDto, { title: '' })).toContain('title');
  });

  it('allows clearing the due date with an empty string', async () => {
    expect(await failedFields(UpdateTaskDto, { dueDate: '' })).toEqual([]);
  });
});

describe('ListTasksDto', () => {
  it('accepts a combined filter, search, sort and pagination query', async () => {
    const fields = await failedFields(ListTasksDto, {
      status: 'todo',
      search: 'alpha',
      sortBy: 'due_date',
      order: 'asc',
      page: '2',
      limit: '10',
    });
    expect(fields).toEqual([]);
  });

  it('rejects invalid sort, order and pagination values', async () => {
    expect(await failedFields(ListTasksDto, { sortBy: 'title' })).toContain(
      'sortBy',
    );
    expect(await failedFields(ListTasksDto, { order: 'up' })).toContain(
      'order',
    );
    expect(await failedFields(ListTasksDto, { page: '0' })).toContain('page');
    expect(await failedFields(ListTasksDto, { page: 'abc' })).toContain('page');
    expect(await failedFields(ListTasksDto, { limit: '101' })).toContain(
      'limit',
    );
  });
});

describe('SignupDto and LoginDto', () => {
  it('accepts a valid signup', async () => {
    expect(
      await failedFields(SignupDto, {
        name: 'Ada',
        email: 'ada@example.com',
        password: 'password123',
      }),
    ).toEqual([]);
  });

  it('requires name, valid email and a long-enough password', async () => {
    expect(
      await failedFields(SignupDto, {
        email: 'a@b.co',
        password: 'longenough',
      }),
    ).toContain('name');
    expect(
      await failedFields(SignupDto, {
        name: 'Ada',
        email: 'nope',
        password: 'longenough',
      }),
    ).toContain('email');
    expect(
      await failedFields(SignupDto, {
        name: 'Ada',
        email: 'a@b.co',
        password: 'short',
      }),
    ).toContain('password');
  });

  it('requires a valid email on login', async () => {
    expect(
      await failedFields(LoginDto, { email: 'nope', password: 'x' }),
    ).toContain('email');
  });
});
