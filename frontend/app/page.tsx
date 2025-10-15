import { TaskList } from '@/components/TaskList';
import { fetchTasks } from '@/services/tasks';

//1.- Home renders the dashboard landing page that lists all backend tasks.
export default async function Home() {
  //2.- Retrieve tasks on the server to leverage Next.js streaming and caching.
  const items = await fetchTasks();

  return (
    <div style={{ display: 'grid', gap: '2rem' }}>
      <header style={{ display: 'grid', gap: '0.5rem' }}>
        <h1 className="page-title">Operations Dashboard</h1>
        <p className="page-subtitle">
          {/*3.- Briefly describe how the backend integration powers the interface.*/}
          Data synchronised from the Yamato Go API keeps this overview up to date.
        </p>
      </header>

      <TaskList items={items} />
    </div>
  );
}
