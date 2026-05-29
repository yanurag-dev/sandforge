/**
 * @sandforge/sdk HTTP client
 * Low-level HTTP wrapper for the Sandforge control plane REST API.
 */

import { SandboxError } from "./types";

/**
 * HTTPClient is the low-level HTTP transport for Sandforge API calls.
 */
export class HTTPClient {
  private baseURL: string;
  private fetch: typeof globalThis.fetch;
  private timeoutMs: number;

  constructor(baseURL: string, fetchImpl?: typeof globalThis.fetch, timeoutMs = 60000) {
    this.baseURL = baseURL;
    // Use provided fetch implementation or fall back to global (Node 18+)
    this.fetch = fetchImpl || globalThis.fetch;
    this.timeoutMs = timeoutMs;
  }

  /**
   * do performs a low-level HTTP request, handles JSON marshal/unmarshal,
   * and converts HTTP errors to SandboxError.
   */
  async do<T = void>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const fullURL = this.baseURL + path;

    const options: RequestInit = {
      method,
      headers: {
        "Content-Type": "application/json",
      },
      signal: AbortSignal.timeout(this.timeoutMs),
    };

    if (body !== undefined) {
      options.body = JSON.stringify(body);
    }

    let response: Response;
    try {
      response = await this.fetch(fullURL, options);
    } catch (err) {
      throw new Error(
        `failed to connect to ${fullURL}: ${
          err instanceof Error ? err.message : String(err)
        }`,
      );
    }

    const responseText = await response.text();

    if (!response.ok) {
      // Try to parse error JSON from server
      let errorMessage = `HTTP ${response.status}`;
      if (responseText) {
        try {
          const errorObj = JSON.parse(responseText);
          if (errorObj.error) {
            errorMessage = `HTTP ${response.status}: ${errorObj.error}`;
          }
        } catch {
          // If JSON parse fails, just use the status code
        }
      }
      throw new SandboxError(response.status, errorMessage);
    }

    // If no response expected and body is empty, return void
    if (!responseText) {
      return undefined as T;
    }

    try {
      return JSON.parse(responseText) as T;
    } catch (err) {
      throw new Error(
        `failed to parse response: ${
          err instanceof Error ? err.message : String(err)
        }`,
      );
    }
  }
}
