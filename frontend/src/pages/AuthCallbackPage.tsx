import { Container } from "@mui/material";
import { Navigate } from "react-router-dom";
import { AuthLoading } from "../features/auth/AuthLoading";
import { useAuth } from "../features/auth/auth-context";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

export function AuthCallbackPage() {
  const auth = useAuth();

  if (auth.status === "loading") {
    return <AuthLoading />;
  }
  if (auth.status === "authenticated") {
    return <Navigate to="/ingredients" replace />;
  }
  if (auth.status === "error" && auth.error) {
    return (
      <Container maxWidth="sm" sx={{ py: 8 }}>
        <ApiErrorAlert error={auth.error} onRetry={() => void auth.refresh()} />
      </Container>
    );
  }
  return <Navigate to="/auth/error" replace />;
}
