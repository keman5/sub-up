import { mount } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { describe, expect, it } from "vitest";

import GroupSharedConfigSections from "../GroupSharedConfigSections.vue";
import { createVideoModelPricesForm } from "../groupsVideoModelPricing";

describe("GroupSharedConfigSections video pricing", () => {
  const mountSections = (form: Record<string, unknown>) =>
    mount(GroupSharedConfigSections, {
      props: {
        form: form as never,
        imageFinalPricePreview: [],
        videoFinalPricePreview: [],
        fallbackGroupOptions: [],
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: "en",
            messages: { en: {} },
            missingWarn: false,
            fallbackWarn: false,
          }),
        ],
      },
    });

  it("renders and updates the upstream model-family price matrix for Grok", async () => {
    const form = {
      platform: "grok",
      subscription_type: "standard",
      allow_image_generation: false,
      allow_batch_image_generation: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      batch_image_discount_multiplier: 0.5,
      batch_image_hold_multiplier: 0.6,
      image_price_1k: null,
      image_price_2k: null,
      image_price_4k: null,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      video_price_480p: null,
      video_price_720p: null,
      video_price_1080p: null,
      video_model_prices: createVideoModelPricesForm(),
      peak_rate_enabled: false,
      peak_start: "",
      peak_end: "",
      peak_rate_multiplier: 1,
      profit_control_enabled: false,
      profit_min_margin_percent: null,
      profit_safety_buffer_percent: null,
      supported_model_scopes: [],
      mcp_xml_inject: false,
      claude_code_only: false,
      fallback_group_id: null,
    } as const;

    const wrapper = mountSections(form);

    expect(wrapper.get('[data-testid="grok-video-model-prices"]').exists()).toBe(true);
    const input = wrapper.get(
      '[data-testid="grok-video-price-grok-imagine-video-1.5-1080p"]',
    );
    await input.setValue("0.42");
    expect(form.video_model_prices["grok-imagine-video-1.5"]["1080p"]).toBe(0.42);
  });

  it("uses the upstream platform-specific image price defaults for Grok", () => {
    const form = {
      platform: "grok",
      subscription_type: "standard",
      allow_image_generation: false,
      allow_batch_image_generation: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      batch_image_discount_multiplier: 0.5,
      batch_image_hold_multiplier: 0.6,
      image_price_1k: null,
      image_price_2k: null,
      image_price_4k: null,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      video_price_480p: null,
      video_price_720p: null,
      video_price_1080p: null,
      video_model_prices: createVideoModelPricesForm(),
      peak_rate_enabled: false,
      peak_start: "",
      peak_end: "",
      peak_rate_multiplier: 1,
      profit_control_enabled: false,
      profit_min_margin_percent: null,
      profit_safety_buffer_percent: null,
      supported_model_scopes: [],
      mcp_xml_inject: false,
      claude_code_only: false,
      fallback_group_id: null,
    };

    const imageInputs = mountSections(form)
      .findAll('input[type="number"]')
      .slice(0, 3);
    expect(imageInputs.map((input) => input.attributes("placeholder"))).toEqual([
      "0.02",
      "0.02",
      "0.02",
    ]);
  });

  it("preserves Gemini image billing and batch billing interactions", async () => {
    const form = {
      platform: "gemini",
      subscription_type: "standard",
      allow_image_generation: false,
      allow_batch_image_generation: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      batch_image_discount_multiplier: 0.5,
      batch_image_hold_multiplier: 0.6,
      image_price_1k: null,
      image_price_2k: null,
      image_price_4k: null,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      video_price_480p: null,
      video_price_720p: null,
      video_price_1080p: null,
      video_model_prices: createVideoModelPricesForm(),
      peak_rate_enabled: false,
      peak_start: "",
      peak_end: "",
      peak_rate_multiplier: 1,
      profit_control_enabled: false,
      profit_min_margin_percent: null,
      profit_safety_buffer_percent: null,
      supported_model_scopes: [],
      mcp_xml_inject: false,
      claude_code_only: false,
      fallback_group_id: null,
    };
    const wrapper = mountSections(form);

    expect(wrapper.find('[data-testid="grok-video-model-prices"]').exists()).toBe(false);
    await wrapper.get('[data-testid="allow-image-generation"]').setValue(true);
    await wrapper.get('[data-testid="image-rate-independent"]').setValue(true);
    await wrapper.get('[data-testid="image-rate-multiplier"]').setValue("1.25");
    await wrapper.get('[data-testid="image-price-1k"]').setValue("0.11");
    await wrapper.get('[data-testid="image-price-2k"]').setValue("0.22");
    await wrapper.get('[data-testid="image-price-4k"]').setValue("0.44");
    await wrapper.get('[data-testid="allow-batch-image-generation"]').setValue(true);

    expect(form).toMatchObject({
      allow_image_generation: true,
      allow_batch_image_generation: true,
      image_rate_independent: true,
      image_rate_multiplier: 1.25,
      image_price_1k: 0.11,
      image_price_2k: 0.22,
      image_price_4k: 0.44,
    });
  });
});
