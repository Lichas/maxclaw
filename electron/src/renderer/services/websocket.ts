import { store } from '../store';
import { setStatus } from '../store';

type MessageHandler = (data: any) => void;

export type WebSocketMessageType = 'chat' | 'interrupt' | 'stream' | 'status';

export interface WSMessage {
  type: WebSocketMessageType;
  session?: string;
  content?: string;
  mode?: 'cancel' | 'append';
  timestamp?: number;
}

class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private handlers: Map<string, MessageHandler[]> = new Map();
  private url: string;
  private isConnecting = false;

  constructor(url: string = 'ws://localhost:18890/ws') {
    this.url = url;
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.isConnecting) {
      return;
    }

    this.isConnecting = true;

    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.reconnectAttempts = 0;
        this.isConnecting = false;
        store.dispatch(setStatus({ state: 'running', port: 18890 }));
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.isConnecting = false;
        this.attemptReconnect();
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        this.isConnecting = false;
        store.dispatch(setStatus({ state: 'error', port: 18890, error: 'Connection failed' }));
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
      this.isConnecting = false;
      this.attemptReconnect();
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    setTimeout(() => {
      this.connect();
    }, delay);
  }

  private handleMessage(data: string): void {
    try {
      const message = JSON.parse(data);
      const handlers = this.handlers.get(message.type) || [];
      handlers.forEach((handler) => handler(message.payload));
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error);
    }
  }

  on(type: string, handler: MessageHandler): () => void {
    const handlers = this.handlers.get(type) || [];
    handlers.push(handler);
    this.handlers.set(type, handlers);

    // Return unsubscribe function
    return () => {
      const updated = (this.handlers.get(type) || []).filter((h) => h !== handler);
      this.handlers.set(type, updated);
    };
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  send(message: WSMessage): boolean {
    if (!this.isConnected()) {
      console.warn('WebSocket not connected, cannot send message');
      return false;
    }
    try {
      this.ws!.send(JSON.stringify(message));
      return true;
    } catch (error) {
      console.error('Failed to send WebSocket message:', error);
      return false;
    }
  }

  sendChat(session: string, content: string): boolean {
    return this.send({
      type: 'chat',
      session,
      content,
      timestamp: Date.now()
    });
  }

  sendInterrupt(session: string, mode: 'cancel' | 'append', content?: string): boolean {
    return this.send({
      type: 'interrupt',
      session,
      mode,
      content: content || '',
      timestamp: Date.now()
    });
  }
}

export const wsClient = new WebSocketClient();
