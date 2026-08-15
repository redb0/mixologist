import { Stack, Typography } from "@mui/material";
import { useAuth } from "../features/auth/auth-context";

export function UserHomePage() {
  const { user } = useAuth();
  return (
    <Stack spacing={1}>
      <Typography variant="h4" component="h1">
        Добро пожаловать, {user?.display_name}
      </Typography>
      <Typography color="text.secondary">
        Управление каталогом ингредиентов доступно только администраторам.
      </Typography>
    </Stack>
  );
}
