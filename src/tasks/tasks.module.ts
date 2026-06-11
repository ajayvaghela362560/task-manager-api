import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';

import { Activity } from '../entities/activity.entity';
import { Attachment } from '../entities/attachment.entity';
import { Task } from '../entities/task.entity';
import { AttachmentsController } from './attachments.controller';
import { AttachmentsService } from './attachments.service';
import { TasksController } from './tasks.controller';
import { TasksService } from './tasks.service';

@Module({
  imports: [TypeOrmModule.forFeature([Task, Activity, Attachment])],
  controllers: [TasksController, AttachmentsController],
  providers: [TasksService, AttachmentsService],
})
export class TasksModule {}
