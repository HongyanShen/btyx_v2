import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Select } from "antd";
import { get, post } from "../api/client";
import { useSession } from "../app/Auth";
import type { MarketingTag } from "../generated/models";

export const marketingTagsKey = ["marketing-tags"];

export function useMarketingTags() {
  const { canCall } = useSession();
  return useQuery({
    queryKey: marketingTagsKey,
    queryFn: () => get<MarketingTag[]>("/marketing-tags"),
    enabled: canCall("GET /marketing-tags"),
  });
}

export function useCreateMarketingTag(onCreated: (tag: MarketingTag) => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      post<MarketingTag>("/marketing-tags", { name }),
    onSuccess: (tag) => {
      queryClient.setQueryData<MarketingTag[]>(marketingTagsKey, (tags = []) =>
        tags.some((entry) => entry.id === tag.id) ? tags : [...tags, tag],
      );
      onCreated(tag);
      void queryClient.invalidateQueries({ queryKey: marketingTagsKey });
    },
  });
}

export function marketingTagOptions(
  tags: MarketingTag[],
  selected: string[] = [],
) {
  const names = new Map(tags.map((tag) => [tag.key, tag.name]));
  for (const key of selected) {
    if (!names.has(key)) names.set(key, key);
  }

  return [...names].map(([value, label]) => ({ value, label }));
}

export function MarketingTagSelect({
  value,
  onChange,
}: {
  value?: string[];
  onChange?: (value: string[]) => void;
}) {
  const { canCall } = useSession();
  const tags = useMarketingTags();
  const hasLibrary = canCall("GET /marketing-tags");
  const options = marketingTagOptions(tags.data ?? [], value);

  return (
    <Select
      mode="tags"
      options={options}
      optionFilterProp="label"
      value={value}
      onChange={(keys: string[]) => {
        const normalized = keys.map((key) => {
          if (options.some((option) => option.value === key)) return key;
          const label = key.replace(/^#/, "");
          return options.find((option) => option.label === label)?.value ?? key;
        });
        onChange?.([...new Set(normalized)]);
      }}
      loading={hasLibrary && tags.isPending}
      placeholder="选择或输入匹配标签"
      notFoundContent={tags.isError ? "标签库加载失败" : "暂无可选标签"}
    />
  );
}
