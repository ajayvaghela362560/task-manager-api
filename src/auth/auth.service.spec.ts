import { ConflictException, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtService } from '@nestjs/jwt';
import { randomUUID } from 'node:crypto';
import type { Repository } from 'typeorm';

import { User } from '../entities/user.entity';
import { AuthService } from './auth.service';

/** Minimal in-memory stand-in for the User repository. */
function fakeUserRepo() {
  const rows: User[] = [];
  const repo = {
    create: (data: Partial<User>) => Object.assign(new User(), data),
    save: (user: User) => {
      if (rows.some((u) => u.email === user.email)) {
        const err = new Error('duplicate key') as Error & {
          driverError: { code: string };
        };
        err.driverError = { code: '23505' };
        return Promise.reject(err);
      }
      user.id = randomUUID();
      user.createdAt = new Date();
      rows.push(user);
      return Promise.resolve(user);
    },
    findOne: ({ where }: { where: Partial<User> }) =>
      Promise.resolve(
        rows.find(
          (u) =>
            (where.email !== undefined && u.email === where.email) ||
            (where.id !== undefined && u.id === where.id),
        ) ?? null,
      ),
  };
  return { repo: repo as unknown as Repository<User>, rows };
}

function makeService(adminEmails: string[] = []) {
  const { repo, rows } = fakeUserRepo();
  const jwt = new JwtService({
    secret: 'test-secret',
    signOptions: { expiresIn: '1h' },
  });
  const config = new ConfigService({ adminEmails });
  return { service: new AuthService(repo, jwt, config), jwt, rows };
}

describe('AuthService', () => {
  it('signs up a user, hashes the password and issues a verifiable token', async () => {
    const { service, jwt, rows } = makeService();
    const result = await service.signup({
      name: 'Ada',
      email: 'Ada@Example.com',
      password: 'password123',
    });

    expect(result.user.email).toBe('ada@example.com'); // normalized
    expect(result.user.role).toBe('user');
    expect(rows[0].passwordHash).not.toContain('password123');

    const claims = jwt.verify<{ sub: string; name: string; role: string }>(
      result.token,
    );
    expect(claims.sub).toBe(result.user.id);
    expect(claims.name).toBe('Ada');
    expect(claims.role).toBe('user');
  });

  it('grants the admin role to configured emails', async () => {
    const { service } = makeService(['admin@example.com']);
    const result = await service.signup({
      name: 'Admin',
      email: 'admin@example.com',
      password: 'password123',
    });
    expect(result.user.role).toBe('admin');
  });

  it('rejects duplicate emails with a conflict', async () => {
    const { service } = makeService();
    const input = {
      name: 'Ada',
      email: 'ada@example.com',
      password: 'password123',
    };
    await service.signup(input);
    await expect(service.signup(input)).rejects.toThrow(ConflictException);
  });

  it('logs in with correct credentials and rejects wrong ones', async () => {
    const { service } = makeService();
    await service.signup({
      name: 'Ada',
      email: 'ada@example.com',
      password: 'password123',
    });

    const result = await service.login({
      email: 'ada@example.com',
      password: 'password123',
    });
    expect(result.token).toBeTruthy();

    await expect(
      service.login({ email: 'ada@example.com', password: 'wrong-password' }),
    ).rejects.toThrow(UnauthorizedException);
    await expect(
      service.login({ email: 'nobody@example.com', password: 'password123' }),
    ).rejects.toThrow(UnauthorizedException);
  });

  it('rejects tampered and expired tokens', () => {
    const { jwt } = makeService();
    const token = jwt.sign({ sub: 'user-1', name: 'Ada', role: 'user' });
    expect(() => jwt.verify<{ sub: string }>(token + 'tampered')).toThrow();

    const expired = jwt.sign({ sub: 'user-1' }, { expiresIn: '-1m' });
    expect(() => jwt.verify<{ sub: string }>(expired)).toThrow();
  });
});
