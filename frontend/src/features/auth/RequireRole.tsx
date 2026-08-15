import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "./auth-context";

export function RequireAdmin() {
  const { user } = useAuth();
  if (user?.role !== "admin") {
    return <Navigate to="/ingredients" replace />;
  }
  return <Outlet />;
}
