import {
  Box,
  Button,
  Container,
  Paper,
  Stack,
  Typography,
} from "@mui/material";
import { Navigate } from "react-router-dom";
import { AuthLoading } from "../features/auth/AuthLoading";
import { useAuth } from "../features/auth/auth-context";
import { googleLoginUrl } from "../shared/api/auth";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

export function LoginPage() {
  const auth = useAuth();

  if (auth.status === "loading") {
    return <AuthLoading />;
  }
  if (auth.status === "authenticated") {
    return <Navigate to="/" replace />;
  }
  if (auth.status === "error" && auth.error) {
    return (
      <Container maxWidth="sm" sx={{ py: 8 }}>
        <ApiErrorAlert error={auth.error} onRetry={() => void auth.refresh()} />
      </Container>
    );
  }

  return (
    <Box sx={{ minHeight: "100vh", display: "grid", placeItems: "center" }}>
      <Container maxWidth="sm">
        <Paper sx={{ p: { xs: 3, md: 4 } }}>
          <Stack spacing={3}>
            <Typography variant="h4" component="h1">
              Вход в Mixologist
            </Typography>
            <Typography color="text.secondary">
              Войдите через Google, чтобы продолжить. Сессия хранится в
              защищённой cookie.
            </Typography>
            <Button
              variant="contained"
              href={googleLoginUrl("/auth/callback")}
              sx={{ alignSelf: "flex-start" }}
            >
              Войти через Google
            </Button>
          </Stack>
        </Paper>
      </Container>
    </Box>
  );
}
