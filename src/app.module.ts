import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { JwtModule } from '@nestjs/jwt';
import { TypeOrmModule } from '@nestjs/typeorm';
import type { StringValue } from 'ms';

import { AuthModule } from './auth/auth.module';
import configuration from './config/configuration';
import { Activity } from './entities/activity.entity';
import { Attachment } from './entities/attachment.entity';
import { Task } from './entities/task.entity';
import { User } from './entities/user.entity';
import { EventsModule } from './events/events.module';
import { HealthController } from './health.controller';
import { InitSchema1765400000000 } from './migrations/1765400000000-init-schema';
import { TasksModule } from './tasks/tasks.module';

@Module({
  imports: [
    ConfigModule.forRoot({ isGlobal: true, load: [configuration] }),
    TypeOrmModule.forRootAsync({
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        type: 'postgres',
        url: config.get<string>('databaseUrl'),
        entities: [User, Task, Activity, Attachment],
        migrations: [InitSchema1765400000000],
        migrationsRun: true,
        synchronize: false,
        // Keep retrying while the Postgres container is still booting.
        retryAttempts: 30,
        retryDelay: 1000,
      }),
    }),
    JwtModule.registerAsync({
      global: true,
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        secret: config.get<string>('jwtSecret'),
        signOptions: {
          expiresIn: (config.get<string>('jwtTtl') ?? '168h') as StringValue,
        },
      }),
    }),
    EventsModule,
    AuthModule,
    TasksModule,
  ],
  controllers: [HealthController],
})
export class AppModule {}
