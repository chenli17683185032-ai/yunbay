export type UserGroupTagOption = {
  value: string
  label: string
}

export function buildUserGroupTagOptions(
  tags: UserGroupTagOption[],
  currentValue?: string,
  currentValueLabel = '当前值'
): UserGroupTagOption[] {
  const options = [...tags]
  const trimmedCurrentValue = currentValue?.trim()
  if (
    trimmedCurrentValue &&
    !options.some((option) => option.value === trimmedCurrentValue)
  ) {
    return [
      {
        value: trimmedCurrentValue,
        label: `${trimmedCurrentValue}（${currentValueLabel}）`,
      },
      ...options,
    ]
  }
  return options
}
