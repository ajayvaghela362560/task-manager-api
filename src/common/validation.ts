import { BadRequestException, ValidationPipe } from '@nestjs/common';
import type { ValidationError } from 'class-validator';

/**
 * Global validation pipe matching the API error contract: 400 with
 * code "validation_error" and a field -> problem map.
 */
export function buildValidationPipe(): ValidationPipe {
  return new ValidationPipe({
    whitelist: true,
    transform: true,
    stopAtFirstError: true,
    exceptionFactory: (errors: ValidationError[]) => {
      const fields: Record<string, string> = {};
      for (const e of errors) {
        fields[e.property] =
          Object.values(e.constraints ?? {})[0] ?? 'is invalid';
      }
      return new BadRequestException({
        code: 'validation_error',
        message: 'Invalid input',
        fields,
      });
    },
  });
}
