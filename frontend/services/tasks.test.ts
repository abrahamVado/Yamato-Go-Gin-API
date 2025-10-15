import { fetchTasks } from './tasks';

describe('fetchTasks', () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    //1.- Restore the original fetch implementation to avoid test pollution.
    globalThis.fetch = originalFetch;
    jest.restoreAllMocks();
  });

  it('returns task items from the backend', async () => {
    //1.- Mock the HTTP response to emulate the Go API payload.
    const tasks = [{ id: 'TASK-1', title: 'Test', status: 'done', priority: 'low', assignee: 'Alex', due_date: '2023-01-01' }];
    const mockFetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: tasks }),
    } as unknown as Response);
    globalThis.fetch = mockFetch as unknown as typeof fetch;

    //2.- Invoke the service and assert the mapped results.
    const result = await fetchTasks();
    expect(result).toEqual(tasks);
    expect(mockFetch).toHaveBeenCalledWith('http://localhost:8080/api/tasks', { cache: 'no-store' });
  });

  it('throws when the backend responds with an error', async () => {
    //1.- Simulate a failing response status to surface the error path.
    const mockFetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 500,
    } as unknown as Response);
    globalThis.fetch = mockFetch as unknown as typeof fetch;

    //2.- Ensure the rejection message contains the failing status code.
    await expect(fetchTasks()).rejects.toThrow('Failed to load tasks: 500');
  });
});
