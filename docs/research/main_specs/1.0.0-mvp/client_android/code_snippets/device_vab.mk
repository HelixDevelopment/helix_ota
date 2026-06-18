# Helix OTA — device makefile fragment enabling Virtual A/B with compression (VABc)
# for the target board. The agent's payload must target the device's enabled
# compression (android15-virtual-ab §7, §12). Enablement on RK3588 / Orange Pi 5 Max
# is UNVERIFIED and must be validated on hardware (build_integration.md §7).

$(call inherit-product, $(SRC_TARGET_DIR)/product/generic_ramdisk.mk)
$(call inherit-product, $(SRC_TARGET_DIR)/product/virtual_ab_ota/vabc_features.mk)

# lz4 = default; very fast compress/decompress, low CPU — good for an SBC.
# zstd = better ratio (per fleet segment with CPU/merge-time budget).
PRODUCT_VIRTUAL_AB_COMPRESSION_METHOD := lz4
PRODUCT_VIRTUAL_AB_COMPRESSION_FACTOR := 65536    # 64k window (default)

# Dynamic partitions (super) are a prerequisite for Virtual A/B.
# BOARD_SUPER_PARTITION_SIZE and update groups must be defined in BoardConfig.mk.

# Kernel must provide CONFIG_DM_SNAPSHOT and (for VABc) CONFIG_DM_USER.
# dm-user is a non-upstream module; if modular, load it in first-stage ramdisk:
# BOARD_GENERIC_RAMDISK_KERNEL_MODULES_LOAD += dm-user.ko
