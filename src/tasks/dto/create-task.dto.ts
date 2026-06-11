import {
  IsIn,
  IsISO8601,
  IsOptional,
  IsString,
  Matches,
  MaxLength,
  ValidateIf,
} from 'class-validator';

import { PRIORITIES, STATUSES } from '../../entities/task.entity';

export class CreateTaskDto {
  @MaxLength(200, { message: 'title must be at most 200 characters' })
  @Matches(/\S/, { message: 'title must not be empty' })
  @IsString({ message: 'title is required' })
  title: string;

  @IsOptional()
  @IsString({ message: 'description must be a string' })
  @MaxLength(5000, { message: 'description must be at most 5000 characters' })
  description?: string;

  @IsOptional()
  @IsIn(STATUSES, { message: 'status must be one of: todo, in_progress, done' })
  status?: string;

  @IsOptional()
  @IsIn(PRIORITIES, { message: 'priority must be one of: low, medium, high' })
  priority?: string;

  // An empty string means "no due date" (and clears it on PATCH).
  @IsOptional()
  @ValidateIf((o: CreateTaskDto) => o.dueDate !== '')
  @IsISO8601(
    { strict: true },
    {
      message:
        'dueDate must be a valid RFC3339 timestamp, e.g. 2026-06-15T17:00:00Z',
    },
  )
  dueDate?: string;
}
