interface FSEntry {
  name: string;
  is_dir: boolean;
  size: number;
}

interface FSStatResult {
  name: string;
  size: number;
  is_dir: boolean;
  mod_time: number;
}

// =============================================================================
// Core
// =============================================================================

declare function call(fn: string, args?: Record<string, unknown>): unknown;
declare function asyncCall(fn: string, args?: Record<string, unknown>): Promise<unknown>;
declare function flushAsync(): void;
declare function runAsync<T>(...promises: Promise<T>[]): Promise<T[]>;

// =============================================================================
// kv
// =============================================================================

interface KVModule {
  get(key: string, defaultValue?: string | null): string | null;
  set(key: string, value: string): string;
  delete(key: string): string;
  asyncGet(key: string, defaultValue?: string | null): Promise<string | null>;
  asyncSet(key: string, value: string): Promise<string>;
  asyncDelete(key: string): Promise<string>;
}

declare const kv: KVModule;

// =============================================================================
// http
// =============================================================================

interface HTTPRequestOptions {
  headers?: Record<string, string>;
  body?: string;
}

declare class HTTPResponse {
  readonly statusCode: number;
  readonly text: string;
  readonly headers: Record<string, string>;
  readonly ok: boolean;
  json(): unknown;
}

interface HTTPModule {
  request(method: string, url: string, options?: HTTPRequestOptions): HTTPResponse;
  get(url: string, options?: HTTPRequestOptions): HTTPResponse;
  post(url: string, options?: HTTPRequestOptions): HTTPResponse;
  put(url: string, options?: HTTPRequestOptions): HTTPResponse;
  patch(url: string, options?: HTTPRequestOptions): HTTPResponse;
  delete(url: string, options?: HTTPRequestOptions): HTTPResponse;
  asyncRequest(method: string, url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
  asyncGet(url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
  asyncPost(url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
  asyncPut(url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
  asyncPatch(url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
  asyncDelete(url: string, options?: HTTPRequestOptions): Promise<HTTPResponse>;
}

declare const http: HTTPModule;

// =============================================================================
// fs
// =============================================================================

interface FSModule {
  readText(path: string): string;
  readJson(path: string): unknown;
  writeText(path: string, content: string): string;
  writeJson(path: string, data: unknown, indent?: number | null): string;
  listdir(path: string): FSEntry[];
  exists(path: string): boolean;
  mkdir(path: string): string;
  remove(path: string): string;
  stat(path: string): FSStatResult;
  asyncReadText(path: string): Promise<string>;
  asyncReadJson(path: string): Promise<unknown>;
  asyncWriteText(path: string, content: string): Promise<string>;
  asyncListdir(path: string): Promise<FSEntry[]>;
  asyncExists(path: string): Promise<boolean>;
  asyncMkdir(path: string): Promise<string>;
  asyncRemove(path: string): Promise<string>;
  asyncStat(path: string): Promise<FSStatResult>;
}

declare const fs: FSModule;

// =============================================================================
// time
// =============================================================================

declare function time_now(): number;
