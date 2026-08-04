import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import {
  Alert,
  Avatar,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  Paper,
  Stack,
  Typography,
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { enqueueSnackbar } from "notistack";
import {
  deleteIngredient,
  getIngredient,
  ingredientIconUrl,
  ingredientKeys,
} from "../features/ingredients/api";
import { ApiError } from "../shared/api/client";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

export function IngredientDetailPage() {
  const { id: idParam } = useParams();
  const id = Number(idParam);
  const isValidID = Number.isInteger(id) && id > 0;
  const [confirmOpen, setConfirmOpen] = useState(false);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ingredientKeys.detail(id),
    queryFn: () => getIngredient(id),
    enabled: isValidID,
  });
  const remove = useMutation({
    mutationFn: () => deleteIngredient(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ingredientKeys.lists() });
      enqueueSnackbar("Ингредиент удалён", { variant: "success" });
      navigate("/ingredients");
    },
    onError: (error) => {
      if (error instanceof ApiError && error.code === "RESOURCE_IN_USE") {
        enqueueSnackbar(
          "Ингредиент используется и не может быть удалён",
          { variant: "error" },
        );
        return;
      }
      enqueueSnackbar(error.message, { variant: "error" });
    },
  });

  if (!isValidID) {
    return <Alert severity="error">Некорректный ID ингредиента</Alert>;
  }
  if (query.isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }
  if (query.isError) {
    const apiError = query.error instanceof ApiError ? query.error : undefined;
    return (
      <Stack spacing={2}>
        {apiError ? (
          <ApiErrorAlert error={apiError} onRetry={() => query.refetch()} />
        ) : (
          <Alert severity="error">{query.error.message}</Alert>
        )}
        <Button
          component={RouterLink}
          to="/ingredients"
          startIcon={<ArrowBackIcon />}
        >
          Вернуться к списку
        </Button>
      </Stack>
    );
  }

  const ingredient = query.data;
  if (!ingredient) {
    return null;
  }

  const fields = [
    ["Тип", ingredient.ingredient_type],
    ["Единица измерения", ingredient.unit_measurement],
    ["Крепость", ingredient.abv],
    [
      "Дата создания",
      new Intl.DateTimeFormat("ru-RU", {
        dateStyle: "long",
        timeStyle: "short",
      }).format(new Date(ingredient.created_at)),
    ],
  ];

  return (
    <Stack spacing={3}>
      <Button
        component={RouterLink}
        to="/ingredients"
        startIcon={<ArrowBackIcon />}
        sx={{ alignSelf: "flex-start" }}
      >
        К списку
      </Button>
      <Paper sx={{ p: { xs: 2, md: 4 } }}>
        <Stack direction={{ xs: "column", sm: "row" }} spacing={3}>
          <Avatar
            src={
              ingredient.has_icon ? ingredientIconUrl(ingredient.id) : undefined
            }
            alt={ingredient.name}
            variant="rounded"
            sx={{ width: 128, height: 128, fontSize: 48 }}
          >
            {ingredient.name.slice(0, 1).toUpperCase()}
          </Avatar>
          <Stack spacing={2} sx={{ flex: 1 }}>
            <Stack
              direction={{ xs: "column", md: "row" }}
              sx={{ justifyContent: "space-between", gap: 2 }}
            >
              <Box>
                <Typography variant="h4" component="h1">
                  {ingredient.name}
                </Typography>
                <Typography color="text.secondary">
                  Ингредиент #{ingredient.id}
                </Typography>
              </Box>
              <Stack direction="row" spacing={1}>
                <Button
                  component={RouterLink}
                  to={`/ingredients/${ingredient.id}/edit`}
                  variant="contained"
                  startIcon={<EditIcon />}
                >
                  Изменить
                </Button>
                <Button
                  color="error"
                  variant="outlined"
                  startIcon={<DeleteIcon />}
                  onClick={() => setConfirmOpen(true)}
                >
                  Удалить
                </Button>
              </Stack>
            </Stack>
            <Divider />
            {fields.map(([label, value]) => (
              <Box key={label}>
                <Typography variant="caption" color="text.secondary">
                  {label}
                </Typography>
                <Typography>{value}</Typography>
              </Box>
            ))}
            <Box>
              <Typography variant="caption" color="text.secondary">
                Описание
              </Typography>
              <Typography sx={{ whiteSpace: "pre-wrap" }}>
                {ingredient.description || "Описание не указано"}
              </Typography>
            </Box>
          </Stack>
        </Stack>
      </Paper>

      <Dialog open={confirmOpen} onClose={() => setConfirmOpen(false)}>
        <DialogTitle>Удалить ингредиент?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            «{ingredient.name}» будет удалён без возможности восстановления.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setConfirmOpen(false)}
            disabled={remove.isPending}
          >
            Отмена
          </Button>
          <Button
            color="error"
            onClick={() => remove.mutate()}
            disabled={remove.isPending}
          >
            {remove.isPending ? "Удаление…" : "Удалить"}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
