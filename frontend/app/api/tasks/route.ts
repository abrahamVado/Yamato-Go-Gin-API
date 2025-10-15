import { NextResponse } from 'next/server';
import { fetchTasks } from '@/services/tasks';

//1.- GET proxies task requests to the Go backend so browser calls avoid CORS issues.
export async function GET() {
  try {
    //2.- Delegate to the shared service to preserve business logic in a single place.
    const items = await fetchTasks();
    //3.- Return the tasks in the same envelope that the frontend expects.
    return NextResponse.json({ items });
  } catch (error) {
    //4.- Report failures with a descriptive error payload.
    return NextResponse.json(
      { message: error instanceof Error ? error.message : 'Unknown error' },
      { status: 502 }
    );
  }
}
