import { useAuthStore } from "@/stores/auth-store";
import { LoginPage } from "./login-page";

interface AuthGuardProps {
  children: React.ReactNode;
  requiredPermissions?: string[];
}

export function AuthGuard({ children, requiredPermissions }: AuthGuardProps) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const permissions = useAuthStore((s) => s.user?.permissions ?? []);

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  if (requiredPermissions && requiredPermissions.length > 0) {
    const hasAllPermissions = requiredPermissions.every((p) =>
      permissions.includes(p),
    );
    if (!hasAllPermissions) {
      return (
        <div className="flex flex-col items-center justify-center min-h-screen gap-2">
          <h1 className="text-2xl font-bold">Access Denied</h1>
          <p className="text-muted-foreground">
            You do not have the required permissions to view this page.
          </p>
        </div>
      );
    }
  }

  return <>{children}</>;
}
