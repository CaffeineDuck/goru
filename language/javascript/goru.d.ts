// Type definitions for goru host functions

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
// kv - Key-Value Store Module
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
// http - HTTP Client Module
// =============================================================================

declare class HTTPResponse {
  readonly statusCode: number;
  readonly text: string;
  readonly ok: boolean;
  json(): unknown;
}

interface HTTPModule {
  get(url: string): HTTPResponse;
  asyncGet(url: string): Promise<HTTPResponse>;
}

declare const http: HTTPModule;

// =============================================================================
// fs - Filesystem Module
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
// Time
// =============================================================================

declare function time_now(): number;

// =============================================================================
// Async Utilities
// =============================================================================

declare function flushAsync(): void;
declare function runAsync<T>(...promises: Promise<T>[]): Promise<T[]>;

// =============================================================================
// Low-level Protocol (for custom host functions)
// =============================================================================

declare function _goru_call(fn: string, args: Record<string, unknown>): unknown;
declare function _asyncCall(fn: string, args: Record<string, unknown>): Promise<unknown>;
