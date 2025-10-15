import Shell from "@/components/secure/shell"
import { AdminRoleForm } from "@/components/secure/roles/AdminRoleForm"
import { PrivateNeumorphicShell } from "@/components/views/private/PrivateNeumorphicShell"
import { PrivateViewLayout } from "@/components/views/private/PrivateViewLayout"

type PageProps = {
  searchParams?: {
    id?: string
  }
}

export default function Page({ searchParams }: PageProps) {
  //1.- Interpret optional ids from the query string to hydrate the form for editing.
  const parsedId = searchParams?.id ? Number(searchParams.id) : undefined
  const roleId = Number.isFinite(parsedId) ? Number(parsedId) : undefined

  //2.- Switch the header label between creation and edition to clarify the current action.
  const layoutTitle = roleId ? "Edit role" : "Create role"

  //3.- Render the shared shell with the private layout so the contextual heading is visible above the form.
  //4.- Place the role form inside the neumorphic shell to keep editing visuals aligned with other private pages.
  return (
    <Shell>
      <PrivateViewLayout title={layoutTitle}>
        <PrivateNeumorphicShell testId="roles-add-edit-neumorphic-shell">
          <AdminRoleForm roleId={roleId} />
        </PrivateNeumorphicShell>
      </PrivateViewLayout>
    </Shell>
  )
}
