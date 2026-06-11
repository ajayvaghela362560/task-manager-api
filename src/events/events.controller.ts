import {
  Controller,
  Get,
  Query,
  Req,
  Res,
  UnauthorizedException,
} from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import type { Request, Response } from 'express';

import { ROLE_ADMIN } from '../entities/user.entity';
import { EventsService, type TaskEvent } from './events.service';

interface JwtClaims {
  sub: string;
  role: string;
}

@Controller('events')
export class EventsController {
  constructor(
    private readonly events: EventsService,
    private readonly jwt: JwtService,
  ) {}

  /**
   * Server-Sent Events stream of task changes. The token travels as a query
   * parameter because the browser EventSource API cannot set headers.
   */
  @Get()
  stream(
    @Query('token') token: string,
    @Req() req: Request,
    @Res() res: Response,
  ) {
    let claims: JwtClaims;
    try {
      claims = this.jwt.verify<JwtClaims>(token ?? '');
    } catch {
      throw new UnauthorizedException({
        code: 'unauthorized',
        message: 'Invalid or expired token',
      });
    }

    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');
    res.flushHeaders();
    res.write(': connected\n\n');

    const sub = this.events.subscribe(
      claims.sub,
      claims.role === ROLE_ADMIN,
      (ev: TaskEvent) => {
        res.write(`data: ${JSON.stringify(ev)}\n\n`);
      },
    );
    const heartbeat = setInterval(() => res.write(': ping\n\n'), 25_000);

    req.on('close', () => {
      clearInterval(heartbeat);
      this.events.unsubscribe(sub);
    });
  }
}
