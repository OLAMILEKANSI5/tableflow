import { useEffect, useState } from "react";
import { useForm } from "@mantine/form";
import { Button, classes, Errors, Input, Switch } from "@tableflow/ui-library";
import { TemplateColumn } from "../../../api/types";
import usePostTemplateColumn from "../../../api/usePostTemplateColumn";
import { TemplateColumnProps } from "../types";
import style from "../style/Form.module.scss";

export default function TemplateColumnForm({
  title = "Template column form",
  column = {} as TemplateColumn,
  onSuccess,
  context,
}: TemplateColumnProps) {
  const isEditForm = !!column?.id;
  const [userModifiedKey, setUserModifiedKey] = useState(false);
  const form = useForm({
    initialValues: {
      template_id: context?.templateId || "",
      name: column?.name || "",
      key: column?.key || "",
      required: column?.required || false,
    },
  });
  const { mutate, isLoading, error, isSuccess } = usePostTemplateColumn(context?.templateId);

  useEffect(() => {
    if (isSuccess && !error && !isLoading && onSuccess) onSuccess();
  }, [isSuccess, error, isLoading]);

  const onSubmit = (values: any) => {
    mutate(values);
  };

  const onNameChange = ({ target }: any) => {
    const { value } = target;
    if (value.length === 0 && form.getInputProps("key").value.length === 0) {
      setUserModifiedKey(false);
    }
    form.setFieldValue("name", value);
    if (!userModifiedKey) {
      let keyValue = value
        .replace(/\s/g, "_")
        .replace(/[^a-zA-Z0-9_]/g, "")
        .toLowerCase();
      form.setFieldValue("key", keyValue);
    }
  };

  const onKeyChange = ({ target }: any) => {
    const { value } = target;
    if (value.length === 0 && form.getInputProps("name").value.length === 0) {
      setUserModifiedKey(false);
    } else {
      setUserModifiedKey(true);
    }
    form.setFieldValue("key", value);
  };

  return (
    <div className={style.container}>
      {title && (
        <div className={style.title}>
          <h2>{title}</h2>
        </div>
      )}
      <form onSubmit={form.onSubmit(onSubmit)} aria-disabled={isLoading}>
        <fieldset disabled={isLoading}>
          <Input
            placeholder={!isEditForm ? "name" : `${column?.name}`}
            label="Column name"
            name="name"
            {...form.getInputProps("name")}
            autoFocus={!isEditForm}
            onChange={onNameChange}
            required
          />
          <Input
            placeholder={!isEditForm ? "key" : `${column?.key}`}
            label="Column key"
            name="key"
            {...form.getInputProps("key")}
            autoFocus={!isEditForm}
            onChange={onKeyChange}
            required
          />
          <label>
            <Switch name="required" {...form.getInputProps("required")} label="Required" inputFirst />
          </label>
        </fieldset>

        <div className={classes([style.actions, style.compact])}>
          <Button type="submit" variants={["primary", "noMargin"]} disabled={isLoading || !form.isDirty()}>
            {isLoading ? "Please wait..." : isEditForm ? "Save column" : "Add"}
          </Button>
        </div>

        <Errors error={error} />
      </form>
    </div>
  );
}
