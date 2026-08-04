import AddIcon from "@mui/icons-material/Add";
import EditIcon from "@mui/icons-material/Edit";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
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
  type GridPaginationModel,
  type GridRenderCellParams,
  type GridSortModel,
} from "@mui/x-data-grid";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link as RouterLink } from "react-router-dom";
import {
  ingredientIconUrl,
  listIngredients,
} from "../features/ingredients/api";
import {
  apiSortToDataGrid,
  dataGridSortToApi,
  DEFAULT_LIST_PARAMS,
  listQueryKey,
} from "../features/ingredients/listState";
import {
  ABV_OPTIONS,
  INGREDIENT_TYPES,
  type Abv,
  type Ingredient,
  type IngredientListParams,
  type IngredientType,
} from "../features/ingredients/types";
import { ApiError } from "../shared/api/client";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = window.setTimeout(() => setDebounced(value), delayMs);
    return () => window.clearTimeout(id);
  }, [value, delayMs]);
  return debounced;
}

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
  const [nameFilter, setNameFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState<IngredientType | "">("");
  const [abvFilter, setAbvFilter] = useState<Abv | "">("");
  const debouncedName = useDebouncedValue(nameFilter, 300);

  const [pageSize, setPageSize] = useState(DEFAULT_LIST_PARAMS.pageSize);
  const [pageIndex, setPageIndex] = useState(0);
  const [pageTokens, setPageTokens] = useState<string[]>([""]);
  const [sortModel, setSortModel] = useState<GridSortModel>(
    apiSortToDataGrid(DEFAULT_LIST_PARAMS.sort, DEFAULT_LIST_PARAMS.order),
  );

  const listParams = useMemo<IngredientListParams>(() => {
    const { sort, order } = dataGridSortToApi(sortModel);
    return {
      pageSize,
      pageToken: pageTokens[pageIndex] ?? "",
      sort,
      order,
      filters: {
        name: debouncedName.trim() || undefined,
        ingredient_type: typeFilter || undefined,
        abv: abvFilter || undefined,
      },
    };
  }, [
    abvFilter,
    debouncedName,
    pageIndex,
    pageSize,
    pageTokens,
    sortModel,
    typeFilter,
  ]);

  useEffect(() => {
    setPageIndex(0);
    setPageTokens([""]);
  }, [debouncedName, typeFilter, abvFilter, sortModel, pageSize]);

  const query = useQuery({
    queryKey: listQueryKey(listParams),
    queryFn: () => listIngredients(listParams),
    placeholderData: keepPreviousData,
  });

  const handlePaginationModelChange = (model: GridPaginationModel) => {
    if (model.pageSize !== pageSize) {
      setPageSize(model.pageSize);
      return;
    }
    if (model.page > pageIndex) {
      const nextToken = query.data?.nextPageToken;
      if (!nextToken) {
        return;
      }
      setPageTokens((tokens) => [...tokens, nextToken]);
      setPageIndex(model.page);
      return;
    }
    if (model.page < pageIndex) {
      setPageIndex(model.page);
    }
  };

  const handleSortModelChange = (model: GridSortModel) => {
    if (model.length > 1) {
      return;
    }
    setSortModel(model);
  };

  const apiError = query.error instanceof ApiError ? query.error : undefined;

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
          value={nameFilter}
          onChange={(event) => setNameFilter(event.target.value)}
          sx={{ minWidth: 240, flex: 1 }}
        />
        <FormControl sx={{ minWidth: 220 }}>
          <InputLabel>Тип</InputLabel>
          <Select
            value={typeFilter}
            label="Тип"
            onChange={(event) =>
              setTypeFilter(event.target.value as IngredientType | "")
            }
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
            value={abvFilter}
            label="Крепость"
            onChange={(event) => setAbvFilter(event.target.value as Abv | "")}
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

      {apiError && (
        <ApiErrorAlert error={apiError} onRetry={() => query.refetch()} />
      )}
      <Box sx={{ height: 580, width: "100%" }}>
        <DataGrid
          rows={query.data?.ingredients ?? []}
          columns={columns}
          loading={query.isLoading}
          disableRowSelectionOnClick
          paginationMode="server"
          sortingMode="server"
          rowCount={query.data?.totalSize ?? 0}
          pageSizeOptions={[10, 25, 50]}
          paginationModel={{ page: pageIndex, pageSize }}
          onPaginationModelChange={handlePaginationModelChange}
          sortModel={sortModel}
          onSortModelChange={handleSortModelChange}
          sortingOrder={["asc", "desc"]}
          localeText={{ noRowsLabel: "Ингредиенты не найдены" }}
        />
      </Box>
    </Stack>
  );
}
