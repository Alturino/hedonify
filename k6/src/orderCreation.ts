import http from "k6/http";
import { check, fail, sleep } from "k6";
import { SharedArray } from "k6/data";
import { Counter } from "k6/metrics";

const minQuantity = 1;
const maxQuantity = 10;
const minOrderItems = 1;
const maxOrderItems = 5;

export const options = {
  vus: 10,
  duration: "5s",
};

const registers = new SharedArray("registers", () => {
  return JSON.parse(open("./registers.json"));
});

const users = new SharedArray("users", () => {
  return JSON.parse(open("./users.json"));
});

const productsSeed = new SharedArray("products", () => {
  return JSON.parse(open("./products.json"));
});

const counterOrderSuccess = new Counter("order_success");
const counterOrderFail = new Counter("order_fail");

export default function () {
  // randomize user
  const userRandomIndex = Math.floor(Math.random() * users.length);
  const register = registers[userRandomIndex];
  const registerResponse = http
    .post("http://localhost/users/register", JSON.stringify(register), {})
    .json();
  if (
    !check(registerResponse, {
      "Success registering user": (r) =>
        r?.statusCode === 200 || r?.statusCode === 409,
    })
  ) {
    console.error(`registerResponse=${JSON.stringify(registerResponse)}`);
    fail("Failed to register user");
  }
  console.log(`registerResponse=${JSON.stringify(registerResponse)}`);

  const user = users[userRandomIndex];
  const loginResponse = http
    .post("http://localhost/users/login", JSON.stringify(user), {})
    .json();
  const isLoginSuccess = check(loginResponse, {
    "Success login user": (r) => r?.statusCode === 200,
  });
  if (!isLoginSuccess) {
    console.error(`loginResponse=${JSON.stringify(loginResponse)}`);
    fail("Failed login user");
  }
  console.log(`loginResponse=${JSON.stringify(loginResponse)}`);
  const token = loginResponse.data.token;

  // insert products
  const insertProductRequest = [];
  for (const product of productsSeed) {
    insertProductRequest.push({
      method: "POST",
      url: "http://localhost/products",
      body: JSON.stringify(product),
      params: {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      },
    });
  }
  console.log(`insertProductRequest=${JSON.stringify(insertProductRequest)}`);
  const response = http.batch(insertProductRequest).map((r) => r.json());
  console.log(`insertProductResponse=${JSON.stringify(response)}`);

  // randomize product selection
  const productResponse = http
    .get("http://localhost/products", {
      headers: { "Content-Type": "application/json" },
    })
    .json();
  const isProductSuccess = check(productResponse, {
    "Success get products": (r) => r?.statusCode === 200,
  });
  if (!isProductSuccess) {
    console.error(`productResponse=${JSON.stringify(productResponse)}`);
    fail("Failed get products");
  }
  console.log(`productResponse=${JSON.stringify(productResponse)}`);
  const cartItems = [];
  const products = productResponse?.data.products;
  const randomOrderItemCount = Math.floor(
    Math.random() * (maxOrderItems - minOrderItems) + minOrderItems,
  );
  for (let i = 0; i < randomOrderItemCount; i++) {
    const product = products[Math.floor(Math.random() * products.length)];
    const newProduct = {
      price: product.price,
      product_id: product.id,
      quantity: Math.floor(
        Math.random() * (maxQuantity - minQuantity) + minQuantity,
      ),
    };
    cartItems.push(newProduct);
  }
  console.log(`cartItems=${JSON.stringify(cartItems)}`);

  // insert cart
  const insertCartResponse = http
    .post("http://localhost/carts", JSON.stringify({ cart_items: cartItems }), {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    })
    .json();
  const isInsertCartSuccess = check(insertCartResponse, {
    "Success insert cart": (r) => r?.statusCode === 200,
  });
  if (!isInsertCartSuccess) {
    console.error(`insertCartResponse=${JSON.stringify(insertCartResponse)}`);
    fail("Failed insert cart");
  }
  console.log(`insertCartResponse=${JSON.stringify(insertCartResponse)}`);

  // checkout cart
  const cartId = insertCartResponse.data.cart.id;
  const checkoutResponse = http
    .post(`http://localhost/carts/${cartId}/checkout`, JSON.stringify({}), {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    })
    .json();
  const isCheckoutSuccess = check(checkoutResponse, {
    "Success checkout cart": (r) => r?.statusCode === 200,
  });
  if (!isCheckoutSuccess) {
    counterOrderFail.add(1);
    console.error(`checkoutResponse=${JSON.stringify(checkoutResponse)}`);
    fail("Failed checkout");
  }
  console.log(`checkoutResponse=${JSON.stringify(checkoutResponse)}`);
  counterOrderSuccess.add(1);
}
