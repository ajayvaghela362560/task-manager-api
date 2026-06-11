import {
  ArgumentsHost,
  Catch,
  ExceptionFilter,
  HttpException,
  Logger,
} from '@nestjs/common';
import type { Response } from 'express';

interface ErrorBody {
  code: string;
  message: string;
  fields?: Record<string, string>;
}

const DEFAULT_CODES: Record<number, string> = {
  400: 'bad_request',
  401: 'unauthorized',
  403: 'forbidden',
  404: 'not_found',
  405: 'method_not_allowed',
  409: 'conflict',
  413: 'upload_too_large',
  500: 'internal_error',
};

/**
 * Serializes every error into the API's single envelope:
 *   { "error": { "code", "message", "fields"? } }
 */
@Catch()
export class AllExceptionsFilter implements ExceptionFilter {
  private readonly logger = new Logger(AllExceptionsFilter.name);

  catch(exception: unknown, host: ArgumentsHost) {
    const res = host.switchToHttp().getResponse<Response>();

    if (exception instanceof HttpException) {
      const status = exception.getStatus();
      const payload = exception.getResponse();

      // Handlers throw exceptions with a pre-shaped {code, message, fields?} body.
      if (
        typeof payload === 'object' &&
        payload !== null &&
        'code' in payload
      ) {
        res.status(status).json({ error: payload as ErrorBody });
        return;
      }

      let message = exception.message;
      if (typeof payload === 'string') {
        message = payload;
      } else if (
        typeof payload === 'object' &&
        payload !== null &&
        'message' in payload
      ) {
        const m = (payload as { message: string | string[] }).message;
        message = Array.isArray(m) ? m[0] : m;
      }
      if (status === 413) {
        message = 'File exceeds the 5 MB limit';
      }
      res.status(status).json({
        error: { code: DEFAULT_CODES[status] ?? 'error', message },
      });
      return;
    }

    this.logger.error(
      exception instanceof Error ? exception.stack : String(exception),
    );
    res.status(500).json({
      error: { code: 'internal_error', message: 'Something went wrong' },
    });
  }
}
