import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import type { Request } from 'express';

import type { AuthUser } from './auth-user';

interface JwtClaims {
  sub: string;
  name: string;
  role: string;
}

@Injectable()
export class JwtAuthGuard implements CanActivate {
  constructor(private readonly jwt: JwtService) {}

  canActivate(ctx: ExecutionContext): boolean {
    const req = ctx.switchToHttp().getRequest<Request & { user: AuthUser }>();
    const header = req.headers.authorization ?? '';
    const [scheme, token] = header.split(' ');
    if (scheme !== 'Bearer' || !token) {
      throw new UnauthorizedException({
        code: 'unauthorized',
        message: 'Missing or malformed Authorization header',
      });
    }
    try {
      const claims = this.jwt.verify<JwtClaims>(token);
      req.user = { id: claims.sub, name: claims.name, role: claims.role };
    } catch {
      throw new UnauthorizedException({
        code: 'unauthorized',
        message: 'Invalid or expired token',
      });
    }
    return true;
  }
}
