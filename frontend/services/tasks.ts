import { BACKEND_API_URL } from '@/lib/config';
import type { Task } from '@/types/task';

interface TaskResponse {
  items: Task[];
}

//1.- fetchTasks retrieves the dashboard tasks from the Go API.
export async function fetchTasks(): Promise<Task[]> {
  //2.- Compose the request URL against the backend service.
  const url = `${BACKEND_API_URL}/api/tasks`;

  //3.- Execute the HTTP request while disabling cache to always show fresh data.
  const response = await fetch(url, { cache: 'no-store' });

  //4.- Guard against failed responses so UI errors bubble up explicitly.
  if (!response.ok) {
    throw new Error(`Failed to load tasks: ${response.status}`);
  }

  //5.- Parse the JSON payload and normalise missing arrays to an empty list.
  const payload = (await response.json()) as Partial<TaskResponse>;
  return Array.isArray(payload.items) ? payload.items : [];
}
