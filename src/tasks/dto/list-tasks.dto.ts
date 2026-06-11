import { Type } from 'class-transformer';
import { IsIn, IsInt, IsOptional, IsString, Max, Min } from 'class-validator';

import { STATUSES } from '../../entities/task.entity';

export class ListTasksDto {
  @IsOptional()
  @IsIn(STATUSES, { message: 'status must be one of: todo, in_progress, done' })
  status?: string;

  @IsOptional()
  @IsString()
  search?: string;

  @IsOptional()
  @IsIn(['created_at', 'createdAt', 'due_date', 'dueDate', 'priority'], {
    message: 'sortBy must be one of: created_at, due_date, priority',
  })
  sortBy?: string;

  @IsOptional()
  @IsIn(['asc', 'desc'], { message: 'order must be asc or desc' })
  order?: string;

  @IsOptional()
  @Type(() => Number)
  @IsInt({ message: 'page must be a positive integer' })
  @Min(1, { message: 'page must be a positive integer' })
  page?: number;

  @IsOptional()
  @Type(() => Number)
  @IsInt({ message: 'limit must be between 1 and 100' })
  @Min(1, { message: 'limit must be between 1 and 100' })
  @Max(100, { message: 'limit must be between 1 and 100' })
  limit?: number;

  /** "all" lets admins list every user's tasks. */
  @IsOptional()
  @IsString()
  scope?: string;
}
