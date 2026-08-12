# vars for use by utils
# the following are already defined in build/make:
# empty space comma newline pound backslash
colon := $(empty):$(empty)

# $(call match-word,w1,w2)
# checks if w1 == w2
# How it works
#   if (w1-w2 not empty or w2-w1 not empty) then not_match else match
#
# returns true or empty
#$(warning :$(1): :$(2): :$(subst $(1),,$(2)):) \
#$(warning :$(2): :$(1): :$(subst $(2),,$(1)):) \
#
define match-word
$(strip \
  $(if $(or $(subst $(1),$(empty),$(2)),$(subst $(2),$(empty),$(1))),,true) \
)
endef

# $(call find-word-in-list,w,wlist)
# finds an exact match of word w in word list wlist
#
# How it works
#   fill wlist spaces with colon
#   wrap w with colon
#   search word w in list wl, if found match m, return stripped word w
#
# returns stripped word or empty
define find-word-in-list
$(strip \
  $(eval wl:= $(colon)$(subst $(space),$(colon),$(strip $(2)))$(colon)) \
  $(eval w:= $(colon)$(strip $(1))$(colon)) \
  $(eval m:= $(findstring $(w),$(wl))) \
  $(if $(m),$(1),) \
)
endef

# $(call match-word-in-list,w,wlist)
# does an exact match of word w in word list wlist
# How it works
#   if the input word is not empty
#     return output of an exact match of word w in wordlist wlist
#   else
#     return empty
# returns true or empty
define match-word-in-list
$(strip \
  $(if $(strip $(1)), \
    $(call match-word,$(call find-word-in-list,$(1),$(2)),$(strip $(1))), \
  ) \
)
endef

# ----
# The following utilities are meant for board platform specific
# featurisation

ifndef get-vendor-board-platforms
# $(call get-vendor-board-platforms,v)
# returns list of board platforms for vendor v
define get-vendor-board-platforms
$(if $(call match-word,$(BOARD_USES_$(1)_HARDWARE),true),$($(1)_BOARD_PLATFORMS))
endef
endif # get-vendor-board-platforms

# $(call is-board-platform,bp)
# returns true or empty
define is-board-platform
$(call match-word,$(1),$(TARGET_BOARD_PLATFORM))
endef

# $(call is-board-platform-in-list,bpl)
# returns true or empty
define is-board-platform-in-list
$(call match-word-in-list,$(TARGET_BOARD_PLATFORM),$(1))
endef

# $(call is-vendor-board-platform,vendor)
# returns true or empty
define is-vendor-board-platform
$(strip \
  $(call match-word-in-list,$(TARGET_BOARD_PLATFORM),\
    $(call get-vendor-board-platforms,$(1)) \
  ) \
)
endef

# $(call is-platform-sdk-version-at-least,version)
# version is a numeric SDK version
define is-platform-sdk-version-at-least
$(strip \
  $(if $(filter 1,$(shell echo "$$(( $(PLATFORM_SDK_VERSION) >= $(1) ))" )), \
    true, \
  ) \
)
endef

# $(call is-version-greater-or-equal,version_a,version_b)
# version_a >= version_b
define is-version-greater-or-equal
$(strip \
  $(eval a_major := $(word 1,$(subst ., ,$(1)))) \
  $(eval a_minor := $(word 2,$(subst ., ,$(1)))) \
  $(eval b_major := $(word 1,$(subst ., ,$(2)))) \
  $(eval b_minor := $(word 2,$(subst ., ,$(2)))) \
  $(if $(call math_gt,$(a_major),$(b_major)),true, \
    $(if $(call math_gt_or_eq,$(a_major),$(b_major)), \
      $(if $(call math_gt_or_eq,$(a_minor),$(b_minor)),true,false), \
    false)) \
)
endef

# $(call is-version-lower-or-equal,version_a,version_b)
# version_a <= version_b
define is-version-lower-or-equal
$(strip \
  $(eval a_major := $(word 1,$(subst ., ,$(1)))) \
  $(eval a_minor := $(word 2,$(subst ., ,$(1)))) \
  $(eval b_major := $(word 1,$(subst ., ,$(2)))) \
  $(eval b_minor := $(word 2,$(subst ., ,$(2)))) \
  $(if $(call math_lt,$(a_major),$(b_major)),true, \
    $(if $(call math_lt_or_eq,$(a_major),$(b_major)), \
      $(if $(call math_lt_or_eq,$(a_minor),$(b_minor)),true,false), \
    false)) \
)
endef

# $(call add-radio-file-sha1-checked,path,sha1)
define add-radio-file-sha1-checked
  $(eval path := $(LOCAL_PATH)/$(1))
  $(eval sha1 := $(shell sha1sum "$(path)" | cut -d" " -f 1))
  $(if $(filter $(sha1),$(2)),
    $(call add-radio-file,$(1)),
    $(error $(path) SHA1 mismatch ($(sha1) != $(2))))
endef
