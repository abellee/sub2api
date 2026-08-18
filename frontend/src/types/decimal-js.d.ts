declare module 'decimal.js' {
  class Decimal {
    constructor(value: Decimal.Value)
    isFinite(): boolean
    mul(value: Decimal.Value): Decimal
    toFixed(decimalPlaces?: number): string
    toNumber(): number
  }

  namespace Decimal {
    type Value = string | number | Decimal
  }

  export default Decimal
}
