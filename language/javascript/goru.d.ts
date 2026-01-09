// Type definitions for goru host functions

interface HTTPResponse {
  status: number;
  body: string;
}

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

declare function kv_get(key: string): string | null;
declare function kv_set(key: string, value: string): string;
declare function kv_delete(key: string): string;
declare function http_get(url: string): HTTPResponse;
declare function fs_read(path: string): string;
declare function fs_write(path: string, content: string): string;
declare function fs_list(path: string): FSEntry[];
declare function fs_exists(path: string): boolean;
declare function fs_mkdir(path: string): string;
declare function fs_remove(path: string): string;
declare function fs_stat(path: string): FSStatResult;
declare function _goru_call(fn: string, args: Record<string, unknown>): unknown;
