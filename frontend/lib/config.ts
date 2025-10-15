//1.- BACKEND_API_URL centralises the backend base URL with a sensible development default.
export const BACKEND_API_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() || 'http://localhost:8080';
