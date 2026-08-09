<template>
  <!-- 图片生成计费配置 -->
  <div
    v-if="supportsImagePricingPlatform(form.platform)"
    class="border-t pt-4"
  >
    <label class="mb-2 block font-medium text-gray-700 dark:text-gray-300">
      {{ t(imagePricingI18nKey(form.platform, "title")) }}
    </label>
    <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
      {{ t(imagePricingI18nKey(form.platform, "description")) }}
    </p>
    <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2">
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input
          v-model="form.allow_image_generation"
          type="checkbox"
          data-testid="allow-image-generation"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        {{ t(imagePricingI18nKey(form.platform, "allowImageGeneration")) }}
      </label>
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input
          v-model="form.image_rate_independent"
          type="checkbox"
          data-testid="image-rate-independent"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        {{ t(imagePricingI18nKey(form.platform, "independentMultiplier")) }}
      </label>
    </div>
    <div v-if="form.image_rate_independent" class="mb-4">
      <label class="input-label">{{
        t(imagePricingI18nKey(form.platform, "imageMultiplier"))
      }}</label>
      <input
        v-model.number="form.image_rate_multiplier"
        type="number"
        step="0.0001"
        min="0"
        class="input"
        data-testid="image-rate-multiplier"
        placeholder="1"
      />
    </div>
    <div class="grid grid-cols-3 gap-3">
      <div>
        <label class="input-label">1K ($)</label>
        <input
          v-model.number="form.image_price_1k"
          type="number"
          step="0.001"
          min="0"
          class="input"
          data-testid="image-price-1k"
          :placeholder="getImagePricePlaceholder(form.platform, 'image_price_1k')"
        />
      </div>
      <div>
        <label class="input-label">2K ($)</label>
        <input
          v-model.number="form.image_price_2k"
          type="number"
          step="0.001"
          min="0"
          class="input"
          data-testid="image-price-2k"
          :placeholder="getImagePricePlaceholder(form.platform, 'image_price_2k')"
        />
      </div>
      <div>
        <label class="input-label">4K ($)</label>
        <input
          v-model.number="form.image_price_4k"
          type="number"
          step="0.001"
          min="0"
          class="input"
          data-testid="image-price-4k"
          :placeholder="getImagePricePlaceholder(form.platform, 'image_price_4k')"
        />
      </div>
    </div>
    <div
      v-if="form.platform === 'grok'"
      class="mt-4 border-t border-dashed border-gray-200 pt-4 dark:border-dark-700"
      data-testid="grok-video-model-prices"
    >
      <p class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t("admin.groups.videoPricing.modelOverridesTitle") }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t("admin.groups.videoPricing.modelOverridesDescription") }}
      </p>
      <div class="mt-3 space-y-3">
        <div
          v-for="family in videoModelPriceFamilyRows(form.video_model_prices)"
          :key="family.key"
          class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_repeat(3,minmax(0,7rem))] sm:items-end"
        >
          <div class="min-w-0 pb-1 font-mono text-xs text-gray-700 dark:text-gray-300">
            {{ family.label }}
          </div>
          <label
            v-for="resolution in grokVideoPriceResolutions"
            :key="resolution.key"
            class="block"
          >
            <span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
              {{ resolution.label }} ($/s)
            </span>
            <input
              v-model.number="form.video_model_prices[family.key][resolution.key]"
              type="number"
              step="0.001"
              min="0"
              class="input"
              :data-testid="`grok-video-price-${family.key}-${resolution.key}`"
            />
          </label>
        </div>
      </div>
    </div>
    <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
      {{ t(imagePricingI18nKey(form.platform, "modeHint")) }}
    </p>
    <div class="mt-2 rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-gray-800 dark:text-gray-300">
      <div class="mb-1 font-medium">
        {{ t(imagePricingI18nKey(form.platform, "finalPricePreview")) }}
      </div>
      <div class="grid grid-cols-3 gap-2">
        <div v-for="item in imageFinalPricePreview" :key="item.label">
          {{ item.label }}: {{ item.value }}
        </div>
      </div>
    </div>
    <div
      v-if="form.platform === 'gemini' && form.allow_image_generation"
      class="mt-4 border-t border-dashed border-gray-200 pt-4 dark:border-dark-700"
    >
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input
          v-model="form.allow_batch_image_generation"
          type="checkbox"
          data-testid="allow-batch-image-generation"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        {{ t("admin.groups.imagePricing.allowBatchImageGeneration") }}
      </label>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t("admin.groups.imagePricing.batchSectionHint") }}
      </p>
      <div
        v-if="form.allow_batch_image_generation"
        class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2"
      >
        <div>
          <label class="input-label">{{
            t("admin.groups.imagePricing.batchDiscountMultiplier")
          }}</label>
          <input
            v-model.number="form.batch_image_discount_multiplier"
            type="number"
            step="0.0001"
            min="0"
            class="input"
            placeholder="0.5"
          />
        </div>
        <div>
          <label class="input-label">{{
            t("admin.groups.imagePricing.batchHoldMultiplier")
          }}</label>
          <input
            v-model.number="form.batch_image_hold_multiplier"
            type="number"
            step="0.0001"
            min="0"
            class="input"
            placeholder="0.6"
          />
        </div>
      </div>
    </div>
    <p
      v-else-if="form.platform !== 'gemini'"
      class="mt-4 border-t border-dashed border-gray-200 pt-4 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400"
    >
      {{ t("admin.groups.imagePricing.batchGeminiOnlyHint") }}
    </p>
  </div>

  <!-- 视频生成计费配置（仅 Grok 平台） -->
  <div
    v-if="supportsVideoPricingPlatform(form.platform)"
    class="border-t pt-4"
  >
    <label class="mb-2 block font-medium text-gray-700 dark:text-gray-300">
      {{ t("admin.groups.videoPricing.title") }}
    </label>
    <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
      {{ t("admin.groups.videoPricing.description") }}
    </p>
    <div class="mb-4">
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input
          v-model="form.video_rate_independent"
          type="checkbox"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        {{ t("admin.groups.videoPricing.independentMultiplier") }}
      </label>
    </div>
    <div v-if="form.video_rate_independent" class="mb-4">
      <label class="input-label">{{
        t("admin.groups.videoPricing.videoMultiplier")
      }}</label>
      <input
        v-model.number="form.video_rate_multiplier"
        type="number"
        step="0.0001"
        min="0"
        class="input"
        placeholder="1"
      />
    </div>
    <div class="grid grid-cols-3 gap-3">
      <div>
        <label class="input-label">480p ($/s)</label>
        <input
          v-model.number="form.video_price_480p"
          type="number"
          step="0.001"
          min="0"
          class="input"
          :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_480p')"
        />
      </div>
      <div>
        <label class="input-label">720p ($/s)</label>
        <input
          v-model.number="form.video_price_720p"
          type="number"
          step="0.001"
          min="0"
          class="input"
          :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_720p')"
        />
      </div>
      <div>
        <label class="input-label">1080p ($/s)</label>
        <input
          v-model.number="form.video_price_1080p"
          type="number"
          step="0.001"
          min="0"
          class="input"
          :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_1080p')"
        />
      </div>
    </div>
    <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
      {{ t("admin.groups.videoPricing.modeHint") }}
    </p>
    <div class="mt-2 rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-gray-800 dark:text-gray-300">
      <div class="mb-1 font-medium">
        {{ t("admin.groups.videoPricing.finalPricePreview") }}
      </div>
      <div class="grid grid-cols-3 gap-2">
        <div v-for="item in videoFinalPricePreview" :key="item.label">
          {{ item.label }}: {{ item.value }}
        </div>
      </div>
    </div>
  </div>

  <!-- 高峰时段倍率配置（仅订阅类型分组） -->
  <div v-if="form.subscription_type === 'subscription'" class="border-t pt-4">
    <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2">
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input
          v-model="form.peak_rate_enabled"
          type="checkbox"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        <span>{{ t("admin.groups.peakRate.enable") }}</span>
      </label>
    </div>
    <div v-if="form.peak_rate_enabled" class="mb-4 grid grid-cols-3 gap-3">
      <div>
        <label class="input-label">{{ t("admin.groups.peakRate.peakStart") }}</label>
        <input v-model="form.peak_start" type="time" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t("admin.groups.peakRate.peakEnd") }}</label>
        <input v-model="form.peak_end" type="time" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t("admin.groups.peakRate.peakMultiplier") }}</label>
        <input
          v-model.number="form.peak_rate_multiplier"
          type="number"
          step="0.001"
          min="0"
          class="input"
          placeholder="1"
          :title="t('admin.groups.peakRate.multiplierHint')"
        />
      </div>
    </div>
  </div>

  <div v-if="isProfitControlPlatform(form.platform)" class="border-t pt-4">
    <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
      <input
        v-model="form.profit_control_enabled"
        type="checkbox"
        class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
      />
      <span>{{ t("admin.groups.profitControl.enable") }}</span>
    </label>
    <p class="mb-3 mt-1.5 text-xs text-gray-500 dark:text-gray-400">
      {{
        form.profit_control_enabled
          ? t("admin.groups.profitControl.enabledHint")
          : t("admin.groups.profitControl.disabledHint")
      }}
    </p>
    <div
      v-if="form.profit_control_enabled"
      class="mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2"
    >
      <div>
        <label class="input-label">{{ t("admin.groups.profitControl.minMargin") }}</label>
        <input
          v-model.number="form.profit_min_margin_percent"
          type="number"
          step="0.1"
          min="0"
          max="99.99"
          class="input"
          placeholder="0"
          :title="t('admin.groups.profitControl.minMarginHint')"
        />
      </div>
      <div>
        <label class="input-label">{{ t("admin.groups.profitControl.safetyBuffer") }}</label>
        <input
          v-model.number="form.profit_safety_buffer_percent"
          type="number"
          step="0.1"
          min="0"
          max="99.99"
          class="input"
          placeholder="0"
          :title="t('admin.groups.profitControl.safetyBufferHint')"
        />
      </div>
    </div>
  </div>

  <!-- 支持的模型系列（仅 antigravity 平台） -->
  <div v-if="form.platform === 'antigravity'" class="border-t pt-4">
    <div class="mb-1.5 flex items-center gap-1">
      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t("admin.groups.supportedScopes.title") }}
      </label>
      <div class="group relative inline-flex">
        <Icon
          name="questionCircle"
          size="sm"
          :stroke-width="2"
          class="cursor-help text-gray-400 transition-colors hover:text-primary-500 dark:text-gray-500 dark:hover:text-primary-400"
        />
        <div class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100">
          <div class="rounded-lg bg-gray-900 p-3 text-white shadow-lg dark:bg-gray-800">
            <p class="text-xs leading-relaxed text-gray-300">
              {{ t("admin.groups.supportedScopes.tooltip") }}
            </p>
            <div class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-gray-900 dark:bg-gray-800"></div>
          </div>
        </div>
      </div>
    </div>
    <div class="space-y-2">
      <label
        v-for="scope in supportedScopes"
        :key="scope.value"
        class="flex cursor-pointer items-center gap-2"
      >
        <input
          type="checkbox"
          :checked="form.supported_model_scopes.includes(scope.value)"
          :data-testid="`supported-scope-${scope.value}`"
          class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
          @change="toggleScope(scope.value)"
        />
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ t(scope.labelKey) }}</span>
      </label>
    </div>
    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ t("admin.groups.supportedScopes.hint") }}
    </p>
  </div>

  <!-- MCP XML 协议注入（仅 antigravity 平台） -->
  <div v-if="form.platform === 'antigravity'" class="border-t pt-4">
    <div class="mb-1.5 flex items-center gap-1">
      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t("admin.groups.mcpXml.title") }}
      </label>
      <div class="group relative inline-flex">
        <Icon
          name="questionCircle"
          size="sm"
          :stroke-width="2"
          class="cursor-help text-gray-400 transition-colors hover:text-primary-500 dark:text-gray-500 dark:hover:text-primary-400"
        />
        <div class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100">
          <div class="rounded-lg bg-gray-900 p-3 text-white shadow-lg dark:bg-gray-800">
            <p class="text-xs leading-relaxed text-gray-300">
              {{ t("admin.groups.mcpXml.tooltip") }}
            </p>
            <div class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-gray-900 dark:bg-gray-800"></div>
          </div>
        </div>
      </div>
    </div>
    <div class="flex items-center gap-3">
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
          form.mcp_xml_inject ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
        ]"
        @click="form.mcp_xml_inject = !form.mcp_xml_inject"
      >
        <span
          :class="[
            'inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform',
            form.mcp_xml_inject ? 'translate-x-6' : 'translate-x-1',
          ]"
        />
      </button>
      <span class="text-sm text-gray-500 dark:text-gray-400">
        {{
          form.mcp_xml_inject
            ? t("admin.groups.mcpXml.enabled")
            : t("admin.groups.mcpXml.disabled")
        }}
      </span>
    </div>
  </div>

  <!-- Claude Code 客户端限制（仅 anthropic 平台） -->
  <div v-if="form.platform === 'anthropic'" class="border-t pt-4">
    <div class="mb-1.5 flex items-center gap-1">
      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t("admin.groups.claudeCode.title") }}
      </label>
      <div class="group relative inline-flex">
        <Icon
          name="questionCircle"
          size="sm"
          :stroke-width="2"
          class="cursor-help text-gray-400 transition-colors hover:text-primary-500 dark:text-gray-500 dark:hover:text-primary-400"
        />
        <div class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100">
          <div class="rounded-lg bg-gray-900 p-3 text-white shadow-lg dark:bg-gray-800">
            <p class="text-xs leading-relaxed text-gray-300">
              {{ t("admin.groups.claudeCode.tooltip") }}
            </p>
            <div class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-gray-900 dark:bg-gray-800"></div>
          </div>
        </div>
      </div>
    </div>
    <div class="flex items-center gap-3">
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
          form.claude_code_only ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
        ]"
        @click="form.claude_code_only = !form.claude_code_only"
      >
        <span
          :class="[
            'inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform',
            form.claude_code_only ? 'translate-x-6' : 'translate-x-1',
          ]"
        />
      </button>
      <span class="text-sm text-gray-500 dark:text-gray-400">
        {{
          form.claude_code_only
            ? t("admin.groups.claudeCode.enabled")
            : t("admin.groups.claudeCode.disabled")
        }}
      </span>
    </div>
    <div v-if="form.claude_code_only" class="mt-3">
      <label class="input-label">{{
        t("admin.groups.claudeCode.fallbackGroup")
      }}</label>
      <Select
        v-model="form.fallback_group_id"
        :options="fallbackGroupOptions"
        :placeholder="t('admin.groups.claudeCode.noFallback')"
      />
      <p class="input-hint">
        {{ t("admin.groups.claudeCode.fallbackHint") }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { GroupPlatform, SelectOption, SubscriptionType } from "@/types";
import Select from "@/components/common/Select.vue";
import Icon from "@/components/icons/Icon.vue";
import {
  getImagePricePlaceholder,
  getVideoPricePlaceholder,
  imagePricingI18nKey,
  supportsImagePricingPlatform,
  supportsVideoPricingPlatform,
} from "./groupsImagePricing";
import { isProfitControlPlatform } from "./groupsProfitControl";
import {
  grokVideoPriceResolutions,
  videoModelPriceFamilyRows,
  type VideoModelPricesForm,
} from "./groupsVideoModelPricing";

type ImageFinalPricePreviewItem = {
  label: string;
  value: string;
};

type VideoFinalPricePreviewItem = {
  label: string;
  value: string;
};

export type GroupSharedConfigForm = {
  platform: GroupPlatform;
  subscription_type: SubscriptionType;
  allow_image_generation: boolean;
  allow_batch_image_generation: boolean;
  image_rate_independent: boolean;
  image_rate_multiplier: number;
  batch_image_discount_multiplier: number;
  batch_image_hold_multiplier: number;
  image_price_1k: number | string | null;
  image_price_2k: number | string | null;
  image_price_4k: number | string | null;
  video_rate_independent: boolean;
  video_rate_multiplier: number;
  video_price_480p: number | string | null;
  video_price_720p: number | string | null;
  video_price_1080p: number | string | null;
  video_model_prices: VideoModelPricesForm;
  peak_rate_enabled: boolean;
  peak_start: string;
  peak_end: string;
  peak_rate_multiplier: number;
  profit_control_enabled: boolean;
  profit_min_margin_percent: number | string | null;
  profit_safety_buffer_percent: number | string | null;
  supported_model_scopes: string[];
  mcp_xml_inject: boolean;
  claude_code_only: boolean;
  fallback_group_id: number | null;
};

const props = defineProps<{
  form: GroupSharedConfigForm;
  imageFinalPricePreview: ImageFinalPricePreviewItem[];
  videoFinalPricePreview: VideoFinalPricePreviewItem[];
  fallbackGroupOptions: SelectOption[];
}>();

const { t } = useI18n();
const form = computed(() => props.form);

const supportedScopes = [
  { value: "claude", labelKey: "admin.groups.supportedScopes.claude" },
  { value: "gemini_text", labelKey: "admin.groups.supportedScopes.geminiText" },
  { value: "gemini_image", labelKey: "admin.groups.supportedScopes.geminiImage" },
];

const toggleScope = (scope: string) => {
  const index = form.value.supported_model_scopes.indexOf(scope);
  if (index === -1) {
    form.value.supported_model_scopes.push(scope);
    return;
  }
  form.value.supported_model_scopes.splice(index, 1);
};
</script>
