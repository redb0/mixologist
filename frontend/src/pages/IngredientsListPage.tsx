import AddIcon from "@mui/icons-material/Add";
import EditIcon from "@mui/icons-material/Edit";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
  Alert,
  Avatar,
  Box,
  Button,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Link as RouterLink } from "react-router-dom";
import {
  ingredientIconUrl,
  ingredientKeys,
  listIngredients,
} from "../features/ingredients/api";
import {
  ABV_OPTIONS,
  INGREDIENT_TYPES,
  type Ingredient,
} from "../features/ingredients/types";

const columns: GridColDef<Ingredient>[] = [
  {
    field: "icon",
    headerName: "",
    width: 64,
    sortable: false,
    filterable: false,
    renderCell: ({ row }: GridRenderCellParams<Ingredient>) => (
      <Avatar
        src={row.has_icon ? ingredientIconUrl(row.id) : undefined}
        alt={row.name}
        variant="rounded"
      >
        {row.name.slice(0, 1).toUpperCase()}
      </Avatar>
    ),
  },
  { field: "name", headerName: "Название", minWidth: 180, flex: 1 },
  { field: "ingredient_type", headerName: "Тип", minWidth: 180, flex: 1 },
  { field: "unit_measurement", headerName: "Единица", width: 110 },
  { field: "abv", headerName: "Крепость", minWidth: 180, flex: 1 },
  {
    field: "created_at",
    headerName: "Создан",
    width: 150,
    renderCell: ({ row }: GridRenderCellParams<Ingredient>) =>
      new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium" }).format(
        new Date(row.created_at),
      ),
  },
  {
    field: "actions",
    headerName: "Действия",
    width: 120,
    sortable: false,
    filterable: false,
    renderCell: ({ row }: GridRenderCellParams<Ingredient>) => (
      <Stack direction="row">
        <Tooltip title="Просмотреть">
          <IconButton
            component={RouterLink}
            to={`/ingredients/${row.id}`}
            aria-label="Просмотреть"
          >
            <VisibilityIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title="Изменить">
          <IconButton
            component={RouterLink}
            to={`/ingredients/${row.id}/edit`}
            aria-label="Изменить"
          >
            <EditIcon />
          </IconButton>
        </Tooltip>
      </Stack>
    ),
  },
];

export function IngredientsListPage() {
  const [search, setSearch] = useState("");
  const [type, setType] = useState("");
  const [abv, setAbv] = useState("");
  const query = useQuery({
    queryKey: ingredientKeys.all,
    queryFn: listIngredients,
  });

  const rows = useMemo(() => {
    const normalizedSearch = search.trim().toLocaleLowerCase("ru");
    return (query.data ?? []).filter(
      (ingredient) =>
        (!normalizedSearch ||
          ingredient.name.toLocaleLowerCase("ru").includes(normalizedSearch)) &&
        (!type || ingredient.ingredient_type === type) &&
        (!abv || ingredient.abv === abv),
    );
  }, [abv, query.data, search, type]);

  return (
    <Stack spacing={3}>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        sx={{ justifyContent: "space-between", gap: 2 }}
      >
        <Box>
          <Typography variant="h4" component="h1">
            Ингредиенты
          </Typography>
          <Typography color="text.secondary">
            Создание и управление ингредиентами каталога
          </Typography>
        </Box>
        <Button
          component={RouterLink}
          to="/ingredients/new"
          variant="contained"
          startIcon={<AddIcon />}
          sx={{ alignSelf: { xs: "stretch", sm: "center" } }}
        >
          Добавить ингредиент
        </Button>
      </Stack>

      <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
        <TextField
          label="Поиск по названию"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          sx={{ minWidth: 240, flex: 1 }}
        />
        <FormControl sx={{ minWidth: 220 }}>
          <InputLabel>Тип</InputLabel>
          <Select
            value={type}
            label="Тип"
            onChange={(event) => setType(event.target.value)}
          >
            <MenuItem value="">Все типы</MenuItem>
            {INGREDIENT_TYPES.map((value) => (
              <MenuItem key={value} value={value}>
                {value}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl sx={{ minWidth: 220 }}>
          <InputLabel>Крепость</InputLabel>
          <Select
            value={abv}
            label="Крепость"
            onChange={(event) => setAbv(event.target.value)}
          >
            <MenuItem value="">Любая крепость</MenuItem>
            {ABV_OPTIONS.map((value) => (
              <MenuItem key={value} value={value}>
                {value}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {query.isError && <Alert severity="error">{query.error.message}</Alert>}
      <Box sx={{ height: 580, width: "100%" }}>
        <DataGrid
          rows={rows}
          columns={columns}
          loading={query.isLoading}
          disableRowSelectionOnClick
          pageSizeOptions={[10, 25, 50]}
          initialState={{
            pagination: { paginationModel: { pageSize: 10, page: 0 } },
          }}
          localeText={{ noRowsLabel: "Ингредиенты не найдены" }}
        />
      </Box>
    </Stack>
  );
}
