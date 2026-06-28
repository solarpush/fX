import { Injectable } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { environment } from '../../../environments/environment';

interface PreviewMessage {
  type: 'compile' | 'result' | 'error';
  data?: any;
  error?: string;
}

@Injectable({
  providedIn: 'root',
})
export class Websocket {
  private socket: WebSocket | null = null;
  private messageSubject = new Subject<PreviewMessage>();

  connect(url: string = environment.wsUrl): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      return;
    }

    this.socket = new WebSocket(url);

    this.socket.onopen = () => {
      console.log('WebSocket connected');
    };

    this.socket.onmessage = (event) => {
      try {
        const message: PreviewMessage = JSON.parse(event.data);
        this.messageSubject.next(message);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.messageSubject.next({
        type: 'error',
        error: 'WebSocket connection error',
      });
    };

    this.socket.onclose = () => {
      console.log('WebSocket disconnected');
      this.socket = null;
    };
  }

  disconnect(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }

  send(message: PreviewMessage): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(message));
    } else {
      console.error('WebSocket is not connected');
    }
  }

  onMessage(): Observable<PreviewMessage> {
    return this.messageSubject.asObservable();
  }

  compilePreview(typstCode: string): void {
    this.send({
      type: 'compile',
      data: { code: typstCode },
    });
  }
}
