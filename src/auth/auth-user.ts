import { createParamDecorator, type ExecutionContext } from '@nestjs/common';
import type { Request } from 'express';

import { ROLE_ADMIN } from '../entities/user.entity';

/** The authenticated caller, decoded from JWT claims by JwtAuthGuard. */
export interface AuthUser {
  id: string;
  name: string;
  role: string;
}

export function isAdmin(user: AuthUser): boolean {
  return user.role === ROLE_ADMIN;
}

export const CurrentUser = createParamDecorator(
  (_data: unknown, ctx: ExecutionContext): AuthUser => {
    const req = ctx.switchToHttp().getRequest<Request & { user: AuthUser }>();
    return req.user;
  },
);
