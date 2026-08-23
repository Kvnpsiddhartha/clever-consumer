import { products } from "../data/products";

export type Availability = "in_stock" | "out_of_stock" | "preorder" | "unknown";

export type Category = "shoes" | "electronics" | "apparel" | "accessories";

export interface Product {
  slug: string;
  name: string;
  brand: string;
  category: Category;
  price: number;
  currency: string;
  images: string[];
  availability: Availability;
  description: string;
  rating: number;
}

export function getAllProducts(): Product[] {
  return products;
}

export function getProductBySlug(slug: string): Product | undefined {
  return products.find((product) => product.slug === slug);
}

export function getProductsByCategory(category: Category): Product[] {
  return products.filter((product) => product.category === category);
}

export const CATEGORIES: Category[] = ["shoes", "electronics", "apparel", "accessories"];

export function categoryLabel(category: Category): string {
  switch (category) {
    case "shoes":
      return "Shoes";
    case "electronics":
      return "Electronics";
    case "apparel":
      return "Apparel";
    case "accessories":
      return "Accessories";
  }
}
