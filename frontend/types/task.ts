//1.- Task represents the payload returned by the Go API.
export interface Task {
  id: string;
  title: string;
  status: string;
  priority: string;
  assignee: string;
  due_date: string;
}
