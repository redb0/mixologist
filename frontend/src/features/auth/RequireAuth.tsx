import { Navigate, Outlet } from "react-router-dom";
import { ApiErrorAlert } from "../../shared/ui/ApiErrorAlert";
import { AuthLoading } from "./AuthLoading";
import { useAuth } from "./auth-context";

export function RequireAuth() {
  const auth = useAuth();

  if (auth.status === "loading") {
    return <AuthLoading />;
  }
  if (auth.status === "error" && auth.error) {
    return (
      <ApiErrorAlert error={auth.error} onRetry={() => void auth.refresh()} />
    );
  }
  if (auth.status === "anonymous") {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}
