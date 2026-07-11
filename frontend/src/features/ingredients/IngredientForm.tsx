import { zodResolver } from "@hookform/resolvers/zod";
import {
  Button,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
} from "@mui/material";
import { Controller, useForm } from "react-hook-form";
import { useEffect } from "react";
import { z } from "zod";
import {
  ABV_OPTIONS,
  INGREDIENT_TYPES,
  UNIT_MEASUREMENTS,
  type IngredientFormValues,
} from "./types";

const ingredientSchema = z.object({
  name: z
    .string()
    .trim()
    .min(3, "Минимум 3 символа")
    .max(512, "Максимум 512 символов"),
  description: z.string().max(1024, "Максимум 1024 символа"),
  unit_measurement: z.enum(UNIT_MEASUREMENTS),
  abv: z.enum(ABV_OPTIONS),
  ingredient_type: z.enum(INGREDIENT_TYPES),
});

interface IngredientFormProps {
  defaultValues: IngredientFormValues;
  isSubmitting: boolean;
  submitLabel: string;
  onSubmit: (values: IngredientFormValues) => Promise<void>;
  onCancel: () => void;
}

export function IngredientForm({
  defaultValues,
  isSubmitting,
  submitLabel,
  onSubmit,
  onCancel,
}: IngredientFormProps) {
  const {
    control,
    handleSubmit,
    register,
    reset,
    formState: { errors },
  } = useForm<IngredientFormValues>({
    resolver: zodResolver(ingredientSchema),
    defaultValues,
  });

  useEffect(() => reset(defaultValues), [defaultValues, reset]);

  return (
    <Stack
      component="form"
      spacing={3}
      onSubmit={handleSubmit(onSubmit)}
      noValidate
    >
      <TextField
        label="Название"
        required
        autoFocus
        error={Boolean(errors.name)}
        helperText={errors.name?.message}
        slotProps={{ htmlInput: { maxLength: 512 } }}
        {...register("name")}
      />
      <TextField
        label="Описание"
        multiline
        minRows={4}
        error={Boolean(errors.description)}
        helperText={errors.description?.message ?? "До 1024 символов"}
        slotProps={{ htmlInput: { maxLength: 1024 } }}
        {...register("description")}
      />
      <Controller
        name="unit_measurement"
        control={control}
        render={({ field }) => (
          <FormControl error={Boolean(errors.unit_measurement)} required>
            <InputLabel>Единица измерения</InputLabel>
            <Select {...field} label="Единица измерения">
              {UNIT_MEASUREMENTS.map((value) => (
                <MenuItem key={value} value={value}>
                  {value}
                </MenuItem>
              ))}
            </Select>
            <FormHelperText>{errors.unit_measurement?.message}</FormHelperText>
          </FormControl>
        )}
      />
      <Controller
        name="abv"
        control={control}
        render={({ field }) => (
          <FormControl error={Boolean(errors.abv)} required>
            <InputLabel>Крепость</InputLabel>
            <Select {...field} label="Крепость">
              {ABV_OPTIONS.map((value) => (
                <MenuItem key={value} value={value}>
                  {value}
                </MenuItem>
              ))}
            </Select>
            <FormHelperText>{errors.abv?.message}</FormHelperText>
          </FormControl>
        )}
      />
      <Controller
        name="ingredient_type"
        control={control}
        render={({ field }) => (
          <FormControl error={Boolean(errors.ingredient_type)} required>
            <InputLabel>Тип ингредиента</InputLabel>
            <Select {...field} label="Тип ингредиента">
              {INGREDIENT_TYPES.map((value) => (
                <MenuItem key={value} value={value}>
                  {value}
                </MenuItem>
              ))}
            </Select>
            <FormHelperText>{errors.ingredient_type?.message}</FormHelperText>
          </FormControl>
        )}
      />
      <Stack direction="row" spacing={2} sx={{ justifyContent: "flex-end" }}>
        <Button type="button" onClick={onCancel} disabled={isSubmitting}>
          Отмена
        </Button>
        <Button type="submit" variant="contained" disabled={isSubmitting}>
          {isSubmitting ? "Сохранение…" : submitLabel}
        </Button>
      </Stack>
    </Stack>
  );
}
