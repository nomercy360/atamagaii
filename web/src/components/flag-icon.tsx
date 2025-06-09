type FlagProps = {
  code: string
  clsName?: string
  width?: number
  height?: number
}

export function FlagIcon({
  code,
  width = 512,
  height = 512,
  clsName,
}: FlagProps) {
  const flags = {
    th: (
      <svg
        width={width}
        height={height}
        viewBox="0 0 512 512"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        class={clsName}
      >
        <rect
          width="512"
          height="512"
          fill="var(--flag-palette-white, #eeeeee)"
        />
        <path
          fill-rule="evenodd"
          clip-rule="evenodd"
          d="M512 0H0V85.3333H512V0ZM512 426.666H0V511.999H512V426.666Z"
          fill="var(--flag-palette-bright-red, #d80027)"
        />
        <rect
          y="170.668"
          width="512"
          height="170.667"
          fill="var(--flag-palette-navy, #002266)"
        />
      </svg>
    ),
    ge: (
      <svg
        width={width}
        height={height}
        viewBox="0 0 512 512"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        class={clsName}
      >
        <rect
          width="512"
          height="512"
          fill="var(--flag-palette-white, #eeeeee)"
        />
        <path
          d="M512 288V224H288V0H224V224H0V288H224V512H288V288H512Z"
          fill="var(--flag-palette-bright-red, #d80027)"
        />
        <path
          fill-rule="evenodd"
          clip-rule="evenodd"
          d="M384 64V96H352V128H384V160H416V128H448V96H416V64H384ZM96 384V352H128V384H160V416H128V448H96V416H64V384H96ZM384 384V352H416V384H448V416H416V448H384V416H352V384H384ZM96 96V64H128V96H160V128H128V160H96V128H64V96H96Z"
          fill="var(--flag-palette-bright-red, #d80027)"
        />
      </svg>
    ),
    jp: (
      <svg
        width={width}
        height={height}
        viewBox="0 0 512 512"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        class={clsName}
      >
        <rect
          width="512"
          height="512"
          fill="var(--flag-palette-white, #eeeeee)"
        />
        <path
          d="M256 368C317.856 368 368 317.856 368 256C368 194.144 317.856 144 256 144C194.144 144 144 194.144 144 256C144 317.856 194.144 368 256 368Z"
          fill="var(--flag-palette-bright-red, #d80027)"
        />
      </svg>
    ),
  }

  return flags[code as keyof typeof flags] || null
}
