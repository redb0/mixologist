import { Container, Stack } from "@mui/material";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AuthLoading } from "../features/auth/AuthLoading";
import { useAuth } from "../features/auth/auth-context";
import { ApiError } from "../shared/api/client";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

export function LogoutPage() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<ApiError | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    void logout()
      .then(() => {
        if (!cancelled) {
          navigate("/login", { replace: true });
        }
      })
      .catch((caught: unknown) => {
        if (cancelled) {
          return;
        }
        setError(
          caught instanceof ApiError
            ? caught
            : new ApiError(500, "UNKNOWN", "Не удалось выйти"),
        );
      });
    return () => {
      cancelled = true;
    };
  }, [attempt, logout, navigate]);

  if (error) {
    return (
      <Container maxWidth="sm" sx={{ py: 8 }}>
        <Stack spacing={2}>
          <ApiErrorAlert
            error={error}
            onRetry={() => {
              setAttempt((current) => current + 1);
            }}
          />
        </Stack>
      </Container>
    );
  }

  return <AuthLoading />;
}
