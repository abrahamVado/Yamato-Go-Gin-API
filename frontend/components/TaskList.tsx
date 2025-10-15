'use client';

import { useMemo } from 'react';
import type { Task } from '@/types/task';

interface TaskListProps {
  items: Task[];
}

//1.- TaskList renders a simple responsive table for dashboard consumption.
export function TaskList({ items }: TaskListProps) {
  //2.- Derive statistics so the dashboard can highlight task totals.
  const summary = useMemo(() => {
    const total = items.length;
    const completed = items.filter((task) => task.status.toLowerCase() === 'done').length;
    return { total, completed };
  }, [items]);

  //3.- Compose the textual summary displayed above the table.
  const summaryLabel = `${summary.completed} completed out of ${summary.total} tasks`;

  return (
    <section className="task-list">
      <header style={{ padding: '1.5rem 1.5rem 0 1.5rem' }}>
        <h2 className="task-list__title">Task Overview</h2>
        <p className="task-list__meta">{summaryLabel}</p>
      </header>

      <div style={{ overflowX: 'auto' }}>
        <table>
          <thead>
            <tr>
              {['ID', 'Title', 'Status', 'Priority', 'Assignee', 'Due'].map((label) => (
                <th key={label}>{label}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((task) => (
              <tr key={task.id}>
                <td>{task.id}</td>
                <td>{task.title}</td>
                <td style={{ textTransform: 'capitalize' }}>{task.status}</td>
                <td>{task.priority}</td>
                <td>{task.assignee}</td>
                <td>{new Date(task.due_date).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
