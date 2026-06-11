import {
  ConflictException,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtService } from '@nestjs/jwt';
import { InjectRepository } from '@nestjs/typeorm';
import * as bcrypt from 'bcryptjs';
import { Repository } from 'typeorm';

import { ROLE_ADMIN, ROLE_USER, User } from '../entities/user.entity';
import { LoginDto } from './dto/login.dto';
import { SignupDto } from './dto/signup.dto';

function isUniqueViolation(err: unknown): boolean {
  return (
    typeof err === 'object' &&
    err !== null &&
    'driverError' in err &&
    (err as { driverError: { code?: string } }).driverError?.code === '23505'
  );
}

export function toUserJson(u: User) {
  return {
    id: u.id,
    name: u.name,
    email: u.email,
    role: u.role,
    createdAt: u.createdAt,
  };
}

@Injectable()
export class AuthService {
  constructor(
    @InjectRepository(User) private readonly users: Repository<User>,
    private readonly jwt: JwtService,
    private readonly config: ConfigService,
  ) {}

  private issueToken(user: User) {
    const token = this.jwt.sign({
      sub: user.id,
      name: user.name,
      role: user.role,
    });
    return { token, user: toUserJson(user) };
  }

  async signup(dto: SignupDto) {
    const name = dto.name.trim();
    const email = dto.email.trim().toLowerCase();
    const adminEmails = this.config.get<string[]>('adminEmails') ?? [];

    const user = this.users.create({
      name,
      email,
      passwordHash: await bcrypt.hash(dto.password, 10),
      role: adminEmails.includes(email) ? ROLE_ADMIN : ROLE_USER,
    });
    try {
      await this.users.save(user);
    } catch (err) {
      if (isUniqueViolation(err)) {
        throw new ConflictException({
          code: 'email_taken',
          message: 'An account with this email already exists',
        });
      }
      throw err;
    }
    return this.issueToken(user);
  }

  async login(dto: LoginDto) {
    const email = dto.email.trim().toLowerCase();
    const user = await this.users.findOne({ where: { email } });
    if (!user || !(await bcrypt.compare(dto.password, user.passwordHash))) {
      throw new UnauthorizedException({
        code: 'invalid_credentials',
        message: 'Incorrect email or password',
      });
    }
    return this.issueToken(user);
  }

  async me(userId: string) {
    const user = await this.users.findOne({ where: { id: userId } });
    if (!user) {
      throw new UnauthorizedException({
        code: 'unauthorized',
        message: 'Account no longer exists',
      });
    }
    return { user: toUserJson(user) };
  }
}
