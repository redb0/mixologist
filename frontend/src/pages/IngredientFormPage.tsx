import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CloudUploadIcon from "@mui/icons-material/CloudUpload";
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
  Paper,
  Stack,
  Typography,
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { enqueueSnackbar } from "notistack";
import { useEffect, useMemo, useState } from "react";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import {
  createIngredient,
  getIngredient,
  ingredientIconUrl,
  ingredientKeys,
  updateIngredient,
  uploadIngredientIcon,
} from "../features/ingredients/api";
import { IngredientForm } from "../features/ingredients/IngredientForm";
import type {
  IngredientFormValues,
  UpdateIngredientRequest,
} from "../features/ingredients/types";
import { EMPTY_INGREDIENT_VALUES } from "../features/ingredients/types";
import { ApiError } from "../shared/api/client";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

const MAX_ICON_SIZE = 512 * 1024;
const ALLOWED_ICON_TYPES = ["image/png", "image/jpeg"];

export function IngredientFormPage() {
  const { id: idParam } = useParams();
  const isEdit = idParam !== undefined;
  const id = Number(idParam);
  const isValidID = !isEdit || (Number.isInteger(id) && id > 0);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [iconVersion, setIconVersion] = useState(0);
  const [selectedIcon, setSelectedIcon] = useState<File>();
  const [previewUrl, setPreviewUrl] = useState<string>();
  const [versionConflictOpen, setVersionConflictOpen] = useState(false);

  useEffect(() => {
    if (!selectedIcon) {
      setPreviewUrl(undefined);
      return;
    }
    const url = URL.createObjectURL(selectedIcon);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [selectedIcon]);

  const query = useQuery({
    queryKey: ingredientKeys.detail(id),
    queryFn: () => getIngredient(id),
    enabled: isEdit && isValidID,
  });

  const defaultValues = useMemo<IngredientFormValues>(() => {
    if (!query.data) {
      return EMPTY_INGREDIENT_VALUES;
    }
    return {
      name: query.data.name,
      description: query.data.description,
      unit_measurement: query.data.unit_measurement,
      abv: query.data.abv,
      ingredient_type: query.data.ingredient_type,
    };
  }, [query.data]);

  const save = useMutation({
    mutationFn: async (values: IngredientFormValues) => {
      if (!isEdit) {
        return createIngredient(values);
      }
      const patch: UpdateIngredientRequest = {
        version: query.data!.version,
      };
      for (const key of Object.keys(values) as (keyof IngredientFormValues)[]) {
        if (values[key] !== defaultValues[key]) {
          Object.assign(patch, { [key]: values[key] });
        }
      }
      if (Object.keys(patch).length === 1) {
        return query.data!;
      }
      return updateIngredient(id, patch);
    },
  });

  const upload = useMutation({
    mutationFn: (file: File) => uploadIngredientIcon(id, file),
    onSuccess: async () => {
      setSelectedIcon(undefined);
      setIconVersion((version) => version + 1);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ingredientKeys.detail(id) }),
        queryClient.invalidateQueries({ queryKey: ingredientKeys.lists() }),
      ]);
      enqueueSnackbar("Иконка обновлена", { variant: "success" });
    },
    onError: (error) => enqueueSnackbar(error.message, { variant: "error" }),
  });

  const submit = async (values: IngredientFormValues) => {
    try {
      const ingredient = await save.mutateAsync(values);
      await queryClient.invalidateQueries({ queryKey: ingredientKeys.lists() });
      queryClient.setQueryData(
        ingredientKeys.detail(ingredient.id),
        ingredient,
      );
      enqueueSnackbar(isEdit ? "Изменения сохранены" : "Ингредиент создан", {
        variant: "success",
      });
      navigate(`/ingredients/${ingredient.id}`);
    } catch (error) {
      if (error instanceof ApiError && error.code === "VERSION_CONFLICT") {
        setVersionConflictOpen(true);
        return;
      }
      if (error instanceof ApiError && error.code === "ALREADY_EXISTS") {
        enqueueSnackbar("Ингредиент с таким именем уже существует", {
          variant: "error",
        });
        return;
      }
      enqueueSnackbar(
        error instanceof Error
          ? error.message
          : "Не удалось сохранить ингредиент",
        { variant: "error" },
      );
    }
  };

  const selectIcon = (file?: File) => {
    if (!file) {
      return;
    }
    if (!ALLOWED_ICON_TYPES.includes(file.type)) {
      enqueueSnackbar("Выберите PNG или JPEG", { variant: "error" });
      return;
    }
    if (file.size > MAX_ICON_SIZE) {
      enqueueSnackbar("Размер иконки не должен превышать 512 KB", {
        variant: "error",
      });
      return;
    }
    setSelectedIcon(file);
  };

  if (!isValidID) {
    return <Alert severity="error">Некорректный ID ингредиента</Alert>;
  }
  if (isEdit && query.isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }
  if (isEdit && query.isError) {
    const apiError = query.error instanceof ApiError ? query.error : undefined;
    return apiError ? (
      <ApiErrorAlert error={apiError} onRetry={() => query.refetch()} />
    ) : (
      <Alert severity="error">{query.error.message}</Alert>
    );
  }

  return (
    <Stack spacing={3}>
      <Button
        component={RouterLink}
        to={isEdit ? `/ingredients/${id}` : "/ingredients"}
        startIcon={<ArrowBackIcon />}
        sx={{ alignSelf: "flex-start" }}
      >
        Назад
      </Button>
      <Box>
        <Typography variant="h4" component="h1">
          {isEdit ? "Изменить ингредиент" : "Новый ингредиент"}
        </Typography>
        <Typography color="text.secondary">
          {isEdit
            ? "Обновите данные ингредиента"
            : "Заполните данные нового ингредиента"}
        </Typography>
      </Box>
      <Paper sx={{ p: { xs: 2, md: 4 }, maxWidth: 760 }}>
        <IngredientForm
          defaultValues={defaultValues}
          isSubmitting={save.isPending}
          submitLabel={isEdit ? "Сохранить" : "Создать"}
          onSubmit={submit}
          onCancel={() =>
            navigate(isEdit ? `/ingredients/${id}` : "/ingredients")
          }
        />
      </Paper>

      {isEdit && query.data && (
        <Paper sx={{ p: { xs: 2, md: 4 }, maxWidth: 760 }}>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={3}
            sx={{ alignItems: "center" }}
          >
            <Avatar
              src={
                previewUrl ??
                (query.data.has_icon
                  ? `${ingredientIconUrl(id)}?v=${iconVersion}`
                  : undefined)
              }
              alt={query.data.name}
              variant="rounded"
              sx={{ width: 96, height: 96, fontSize: 36 }}
            >
              {query.data.name.slice(0, 1).toUpperCase()}
            </Avatar>
            <Box>
              <Typography variant="h6">Иконка</Typography>
              <Typography color="text.secondary" sx={{ mb: 2 }}>
                PNG или JPEG, не более 512 KB
              </Typography>
              <Button
                component="label"
                variant="outlined"
                startIcon={<CloudUploadIcon />}
                disabled={upload.isPending}
              >
                Выбрать файл
                <input
                  hidden
                  type="file"
                  accept="image/png,image/jpeg"
                  onChange={(event) => selectIcon(event.target.files?.[0])}
                />
              </Button>
              <Button
                variant="contained"
                disabled={!selectedIcon || upload.isPending}
                onClick={() => selectedIcon && upload.mutate(selectedIcon)}
                sx={{ ml: 1 }}
              >
                {upload.isPending ? "Загрузка…" : "Загрузить"}
              </Button>
            </Box>
          </Stack>
        </Paper>
      )}

      <Dialog open={versionConflictOpen} onClose={() => setVersionConflictOpen(false)}>
        <DialogTitle>Конфликт версии</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Ингредиент был изменён другим запросом. Загрузить актуальные данные?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setVersionConflictOpen(false)}>Отмена</Button>
          <Button
            onClick={async () => {
              setVersionConflictOpen(false);
              await query.refetch();
            }}
          >
            Обновить данные
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
