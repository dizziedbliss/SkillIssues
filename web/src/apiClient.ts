/**
 * @file Web's one ApiClient instance.
 *
 * `createFetchTransport` talks plain `fetch()` with `credentials: "include"`
 * (the session is an HttpOnly cookie, never a token in JS). Both App.tsx and
 * LandingPage.tsx import this single instance rather than building their own
 * transport, so there is exactly one API origin and one client configuration
 * for the whole web app.
 */
import { createApiClient, createFetchTransport } from "@skillissues/api-client";

export const API_BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8000";

export const apiClient = createApiClient(createFetchTransport(API_BASE_URL));
