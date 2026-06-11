import { Injectable } from '@nestjs/common';

export interface TaskEvent {
  type: 'task.created' | 'task.updated' | 'task.deleted';
  payload: unknown;
}

interface Subscriber {
  userId: string;
  admin: boolean;
  send: (ev: TaskEvent) => void;
}

/** In-memory pub/sub hub that pushes task events to connected SSE clients. */
@Injectable()
export class EventsService {
  private readonly subs = new Set<Subscriber>();

  subscribe(
    userId: string,
    admin: boolean,
    send: (ev: TaskEvent) => void,
  ): Subscriber {
    const sub: Subscriber = { userId, admin, send };
    this.subs.add(sub);
    return sub;
  }

  unsubscribe(sub: Subscriber) {
    this.subs.delete(sub);
  }

  /**
   * Delivers the event to every connection belonging to the task owner and
   * to all connected admins. A failing subscriber never breaks the others.
   */
  publish(ownerId: string, ev: TaskEvent) {
    for (const sub of this.subs) {
      if (sub.userId === ownerId || sub.admin) {
        try {
          sub.send(ev);
        } catch {
          // ignore broken pipes; the close handler cleans up
        }
      }
    }
  }
}
